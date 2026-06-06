package chromium

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// Browser is one Chromium installation: a single UserDataDir holding profiles
// that share a master key. The key is derived once and reused across profiles.
type Browser struct {
	cfg        types.BrowserConfig
	retrievers masterkey.Retrievers
	profiles   []*profile

	keysOnce sync.Once
	keys     masterkey.MasterKeys
}

// NewBrowser discovers the profiles under cfg.UserDataDir, or returns nil if none resolve.
// Call SetRetrievers before Extract to enable decryption.
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

// SetRetrievers wires the per-tier master-key retrievers (V10/V11/V20) used by
// Extract; unused tiers stay nil.
func (b *Browser) SetRetrievers(r masterkey.Retrievers) { b.retrievers = r }

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) BrowserKey() string  { return b.cfg.Key }
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
	masterKeys := b.masterKeys()
	results := make([]types.ExtractResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.ExtractResult{
			Profile: types.Profile{Name: p.name(), Dir: p.profileDir},
			Data:    p.extract(masterKeys, categories),
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

// ExportKeys derives the master keys without extracting. Returns the tiers that succeeded plus a
// joined error for those that failed — partial results matter (a v20-only failure keeps the v10 key).
func (b *Browser) ExportKeys() (masterkey.MasterKeys, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return masterkey.MasterKeys{}, err
	}
	defer session.Cleanup()

	return masterkey.NewMasterKeys(b.retrievers, b.buildHints(session))
}

// masterKeys derives and caches the installation's keys exactly once (sync.Once), so a failure is
// warned once — no cross-profile dedup state needed.
func (b *Browser) masterKeys() masterkey.MasterKeys {
	b.keysOnce.Do(func() {
		masterKeys, err := b.ExportKeys()
		if err != nil {
			log.Warnf("%s: master key retrieval: %v", b.BrowserName(), err)
		}
		b.keys = masterKeys
	})
	return b.keys
}

// buildHints copies Local State into the session temp dir (so Windows DPAPI/ABE retrievers read it
// from a process-owned path) and assembles the Hints. Local State sits at the installation root.
func (b *Browser) buildHints(session *filemanager.Session) masterkey.Hints {
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
	return masterkey.Hints{
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

// resolvedPath holds the absolute path, the slash-relative source path, and the type of a discovered
// source. rel is retained (not just absPath) so archive can reproduce the User Data layout.
type resolvedPath struct {
	absPath string
	rel     string
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
				resolved[cat] = resolvedPath{absPath: abs, rel: sp.rel, isDir: sp.isDir}
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
