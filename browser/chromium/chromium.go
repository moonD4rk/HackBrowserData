package chromium

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// Browser is one Chromium installation: a single UserDataDir holding profiles
// that share a master key. The key is derived once and reused across profiles.
type Browser struct {
	cfg        types.BrowserConfig
	retrievers keyretriever.Retrievers
	profiles   []*profile

	keysOnce sync.Once
	keys     keyretriever.MasterKeys
}

// NewBrowser discovers the Chromium profiles under cfg.UserDataDir and returns
// the installation, or nil if no profile with resolvable sources exists. Call
// SetKeyRetrievers before Extract to enable decryption of sensitive data.
func NewBrowser(cfg types.BrowserConfig) (*Browser, error) {
	sources := sourcesForKind(cfg.Kind)
	extractors := extractorsForKind(cfg.Kind)

	var profiles []*profile
	for _, profileDir := range discoverProfiles(cfg.UserDataDir, sources) {
		sourcePaths := resolveSourcePaths(sources, profileDir)
		if len(sourcePaths) == 0 {
			continue
		}
		profiles = append(profiles, &profile{
			profileDir:  profileDir,
			browserName: cfg.Name,
			kind:        cfg.Kind,
			extractors:  extractors,
			sourcePaths: sourcePaths,
		})
	}
	if len(profiles) == 0 {
		return nil, nil
	}
	return &Browser{cfg: cfg, profiles: profiles}, nil
}

// SetKeyRetrievers wires the per-tier master-key retrievers (V10/V11/V20) used by
// Extract; unused tiers stay nil.
func (b *Browser) SetKeyRetrievers(r keyretriever.Retrievers) { b.retrievers = r }

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) UserDataDir() string { return b.cfg.UserDataDir }

// Profiles returns the identity of every profile in this installation.
func (b *Browser) Profiles() []types.Profile {
	out := make([]types.Profile, 0, len(b.profiles))
	for _, p := range b.profiles {
		out = append(out, types.Profile{Name: p.name(), Dir: p.profileDir})
	}
	return out
}

// Extract derives the installation's master key once, then extracts every profile.
func (b *Browser) Extract(categories []types.Category) ([]types.ExtractResult, error) {
	keys := b.masterKeys()
	results := make([]types.ExtractResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.ExtractResult{
			Profile: types.Profile{Name: p.name(), Dir: p.profileDir},
			Data:    p.extract(keys, categories),
		})
	}
	return results, nil
}

// CountEntries counts entries per category for every profile without decryption.
func (b *Browser) CountEntries(categories []types.Category) ([]types.CountResult, error) {
	results := make([]types.CountResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.CountResult{
			Profile: types.Profile{Name: p.name(), Dir: p.profileDir},
			Counts:  p.count(categories),
		})
	}
	return results, nil
}

// ExportKeys derives the installation's master keys without extraction. Returns
// whatever tiers succeeded plus a joined error describing any failed tiers;
// callers preserve partial results because a Chrome 127+ installation mixes
// v10 + v20 ciphertexts and a v20-only failure must not erase a usable v10 key.
func (b *Browser) ExportKeys() (keyretriever.MasterKeys, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return keyretriever.MasterKeys{}, err
	}
	defer session.Cleanup()

	return keyretriever.NewMasterKeys(b.retrievers, b.buildHints(session))
}

// masterKeys derives the installation's keys exactly once and caches them.
// Because derivation happens a single time per installation, a failure is warned
// exactly once — no cross-profile dedup state is needed.
func (b *Browser) masterKeys() keyretriever.MasterKeys {
	b.keysOnce.Do(func() {
		keys, err := b.ExportKeys()
		if err != nil {
			log.Warnf("%s: master key retrieval: %v", b.BrowserName(), err)
		}
		b.keys = keys
	})
	return b.keys
}

// buildHints acquires Local State (into session.TempDir so Windows DPAPI/ABE
// retrievers can read it from a path the process owns) and assembles per-tier
// retriever hints. Local State lives at the installation root (cfg.UserDataDir)
// in both the multi-profile and flat (Opera) layouts.
func (b *Browser) buildHints(session *filemanager.Session) keyretriever.Hints {
	var localStateDst string
	candidate := filepath.Join(b.cfg.UserDataDir, "Local State")
	if fileutil.FileExists(candidate) {
		dst := filepath.Join(session.TempDir(), "Local State")
		if err := session.Acquire(candidate, dst, false); err != nil {
			log.Debugf("acquire Local State for %s: %v", b.BrowserName(), err)
		} else {
			localStateDst = dst
		}
	}

	abeKey := ""
	if b.cfg.WindowsABE {
		abeKey = b.cfg.Key
	}
	return keyretriever.Hints{
		KeychainLabel:  b.cfg.KeychainLabel,
		WindowsABEKey:  abeKey,
		LocalStatePath: localStateDst,
	}
}

// discoverProfiles lists subdirectories of userDataDir that are valid
// Chromium profile directories. A directory is considered a profile if it
// contains a "Preferences" file, which Chromium creates for every profile.
func discoverProfiles(userDataDir string, sources map[types.Category][]sourcePath) []string {
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		return nil
	}

	var profiles []string
	for _, e := range entries {
		if !e.IsDir() || isSkippedDir(e.Name()) {
			continue
		}
		dir := filepath.Join(userDataDir, e.Name())
		if isProfileDir(dir) {
			profiles = append(profiles, dir)
		}
	}

	// Flat layout fallback (older Opera): data files directly in userDataDir.
	// Opera stores data alongside Local State in userDataDir itself, so check
	// for any known source file instead of Preferences.
	if len(profiles) == 0 && hasAnySource(sources, userDataDir) {
		profiles = append(profiles, userDataDir)
	}
	return profiles
}

// profileMarkers are filenames that identify a directory as a Chromium profile.
// Chromium creates a per-profile preferences file on first use; checking for
// its existence filters out non-profile subdirectories (Crashpad, ShaderCache, etc.).
//
//   - "Preferences"    — standard Chromium and all major forks (Chrome, Edge, Brave, …)
//   - "Preferences_02" — Tencent-based browsers (QQ Browser, Sogou Explorer)
var profileMarkers = []string{
	"Preferences",
	"Preferences_02",
}

// isProfileDir reports whether dir is a valid Chromium profile directory.
func isProfileDir(dir string) bool {
	for _, name := range profileMarkers {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// hasAnySource checks if dir contains at least one source file or directory.
func hasAnySource(sources map[types.Category][]sourcePath, dir string) bool {
	for _, candidates := range sources {
		for _, sp := range candidates {
			abs := filepath.Join(dir, sp.rel)
			if _, err := os.Stat(abs); err == nil {
				return true
			}
		}
	}
	return false
}

// resolvedPath holds the absolute path and type for a discovered source.
type resolvedPath struct {
	absPath string
	isDir   bool
}

// resolveSourcePaths checks which sources actually exist in profileDir.
// Candidates are tried in priority order; the first existing path wins.
func resolveSourcePaths(sources map[types.Category][]sourcePath, profileDir string) map[types.Category]resolvedPath {
	resolved := make(map[types.Category]resolvedPath)
	for cat, candidates := range sources {
		for _, sp := range candidates {
			abs := filepath.Join(profileDir, sp.rel)
			info, err := os.Stat(abs)
			if err != nil {
				continue
			}
			if sp.isDir == info.IsDir() {
				resolved[cat] = resolvedPath{abs, sp.isDir}
				break
			}
		}
	}
	return resolved
}

// isSkippedDir returns true for directory names that should never be
// treated as browser profiles.
func isSkippedDir(name string) bool {
	switch name {
	case "System Profile", "Guest Profile", "Snapshot":
		return true
	}
	return false
}

// Offset from the Chromium epoch (1601-01-01 UTC) to the Unix epoch,
// matching base::Time::kTimeTToMicrosecondsOffset in Chromium.
const chromiumEpochOffsetMicros int64 = 11644473600000000

// timeEpoch converts a Chromium base::Time (μs since 1601 UTC) to UTC.
// Returns zero for non-positive input or out-of-JSON-range values.
func timeEpoch(epoch int64) time.Time {
	if epoch <= 0 {
		return time.Time{}
	}
	t := time.UnixMicro(epoch - chromiumEpochOffsetMicros).UTC()
	if t.Year() < 1 || t.Year() > 9999 {
		return time.Time{}
	}
	return t
}
