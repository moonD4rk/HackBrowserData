package chromium

import (
	"os"
	"path/filepath"
	"time"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// Browser represents a single Chromium profile ready for extraction.
type Browser struct {
	cfg         types.BrowserConfig
	profileDir  string                               // absolute path to profile directory
	retrievers  keyretriever.Retrievers              // per-tier key sources (V10 / V11 / V20; unused tiers nil)
	sources     map[types.Category][]sourcePath      // Category → candidate paths (priority order)
	extractors  map[types.Category]categoryExtractor // Category → custom extract function override
	sourcePaths map[types.Category]resolvedPath      // Category → discovered absolute path
}

// NewBrowsers discovers Chromium profiles under cfg.UserDataDir and returns
// one Browser per profile. Call SetKeyRetrievers on each returned browser before
// Extract to enable decryption of sensitive data (passwords, cookies, etc.).
func NewBrowsers(cfg types.BrowserConfig) ([]*Browser, error) {
	sources := sourcesForKind(cfg.Kind)
	extractors := extractorsForKind(cfg.Kind)

	profileDirs := discoverProfiles(cfg.UserDataDir, sources)
	if len(profileDirs) == 0 {
		return nil, nil
	}

	var browsers []*Browser
	for _, profileDir := range profileDirs {
		sourcePaths := resolveSourcePaths(sources, profileDir)
		if len(sourcePaths) == 0 {
			continue
		}
		browsers = append(browsers, &Browser{
			cfg:         cfg,
			profileDir:  profileDir,
			sources:     sources,
			extractors:  extractors,
			sourcePaths: sourcePaths,
		})
	}
	return browsers, nil
}

// SetKeyRetrievers wires the per-tier master-key retrievers used by Extract. Each slot
// (V10 / V11 / V20) is populated only on platforms where that cipher tier is used:
//
//   - Windows: V10 (DPAPI) + V20 (ABE). V11 nil — Chromium does not emit v11 prefix on Windows.
//   - Linux:   V10 ("peanuts" kV10Key) + V11 (D-Bus Secret Service kV11Key). V20 nil.
//   - macOS:   V10 (Keychain chain). V11 and V20 nil.
//
// Slots are independent — a failure or absence in one tier does not affect others. A single
// Chromium profile can carry mixed cipher-prefix ciphertexts (the motivation for issue #578), so
// every configured retriever runs at extract time and decryptValue picks the matching key per
// ciphertext.
func (b *Browser) SetKeyRetrievers(r keyretriever.Retrievers) {
	b.retrievers = r
}

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) ProfileDir() string  { return b.profileDir }
func (b *Browser) ProfileName() string {
	if b.profileDir == "" {
		return ""
	}
	return filepath.Base(b.profileDir)
}

// Extract copies browser files to a temp directory, retrieves the master key,
// and extracts data for the requested categories.
func (b *Browser) Extract(categories []types.Category) (*types.BrowserData, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	keys := b.getMasterKeys(session)

	data := &types.BrowserData{}
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		b.extractCategory(data, cat, keys, path)
	}
	return data, nil
}

// CountEntries copies browser files to a temp directory and counts entries
// per category without decryption. Much faster than Extract for display-only
// use cases like "list --detail".
func (b *Browser) CountEntries(categories []types.Category) (map[types.Category]int, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	counts := make(map[types.Category]int)
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		counts[cat] = b.countCategory(cat, path)
	}
	return counts, nil
}

// countCategory calls the appropriate count function for a category.
func (b *Browser) countCategory(cat types.Category, path string) int {
	var count int
	var err error
	switch cat {
	case types.Password:
		count, err = countPasswords(path)
	case types.Cookie:
		count, err = countCookies(path)
	case types.History:
		count, err = countHistories(path)
	case types.Download:
		count, err = countDownloads(path)
	case types.Bookmark:
		count, err = countBookmarks(path)
	case types.CreditCard:
		count, err = countCreditCards(path)
	case types.Extension:
		if b.cfg.Kind == types.ChromiumOpera {
			count, err = countOperaExtensions(path)
		} else {
			count, err = countExtensions(path)
		}
	case types.LocalStorage:
		count, err = countLocalStorage(path)
	case types.SessionStorage:
		count, err = countSessionStorage(path)
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
	return count
}

// acquireFiles copies source files to the session temp directory.
func (b *Browser) acquireFiles(session *filemanager.Session, categories []types.Category) map[types.Category]string {
	tempPaths := make(map[types.Category]string)
	for _, cat := range categories {
		rp, ok := b.sourcePaths[cat]
		if !ok {
			continue
		}
		dst := filepath.Join(session.TempDir(), cat.String())
		if err := session.Acquire(rp.absPath, dst, rp.isDir); err != nil {
			log.Debugf("acquire %s: %v", cat, err)
			continue
		}
		tempPaths[cat] = dst
	}
	return tempPaths
}

// getMasterKeys retrieves the Chromium master keys for every configured tier. Chrome mixes
// cipher tiers on the same profile — v20 for new cookies alongside v10 passwords on Windows; v10
// (peanuts) alongside v11 (keyring) on Linux after session-mode changes — so every retriever in
// b.retrievers runs independently and keyretriever.NewMasterKeys assembles the results. Any tier
// key may be nil if its retriever failed or is not configured for this platform; decryptValue
// treats a missing tier key as "that tier cannot decrypt" so partial success is still reported.
func (b *Browser) getMasterKeys(session *filemanager.Session) keyretriever.MasterKeys {
	label := b.BrowserName() + "/" + b.ProfileName()

	// Locate and copy Local State (needed on Windows, ignored on macOS/Linux). Multi-profile
	// layout: Local State is in the parent of profileDir. Flat layout (Opera): Local State is
	// alongside data files in profileDir.
	var localStateDst string
	for _, dir := range []string{filepath.Dir(b.profileDir), b.profileDir} {
		candidate := filepath.Join(dir, "Local State")
		if !fileutil.FileExists(candidate) {
			continue
		}
		dst := filepath.Join(session.TempDir(), "Local State")
		if err := session.Acquire(candidate, dst, false); err != nil {
			log.Debugf("acquire Local State for %s: %v", label, err)
			break
		}
		localStateDst = dst
		break
	}

	keys, err := keyretriever.NewMasterKeys(b.retrievers, b.cfg.Storage, localStateDst)
	if err != nil {
		log.Warnf("%s: master key retrieval: %v", label, err)
	}
	return keys
}

// extractCategory calls the appropriate extract function for a category.
// If a custom extractor is registered for this category (via extractorsForKind),
// it is used instead of the default switch logic.
func (b *Browser) extractCategory(data *types.BrowserData, cat types.Category, keys keyretriever.MasterKeys, path string) {
	if ext, ok := b.extractors[cat]; ok {
		if err := ext.extract(keys, path, data); err != nil {
			log.Debugf("extract %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
		}
		return
	}

	var err error
	switch cat {
	case types.Password:
		data.Passwords, err = extractPasswords(keys, path)
	case types.Cookie:
		data.Cookies, err = extractCookies(keys, path)
	case types.History:
		data.Histories, err = extractHistories(path)
	case types.Download:
		data.Downloads, err = extractDownloads(path)
	case types.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case types.CreditCard:
		data.CreditCards, err = extractCreditCards(keys, path)
	case types.Extension:
		data.Extensions, err = extractExtensions(path)
	case types.LocalStorage:
		data.LocalStorage, err = extractLocalStorage(path)
	case types.SessionStorage:
		data.SessionStorage, err = extractSessionStorage(path)
	}
	if err != nil {
		log.Debugf("extract %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
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

// timeEpoch converts a WebKit/Chromium epoch timestamp (microseconds since
// 1601-01-01) to a time.Time.
func timeEpoch(epoch int64) time.Time {
	maxTime := int64(99633311740000000)
	if epoch > maxTime {
		return time.Date(2049, 1, 1, 1, 1, 1, 1, time.Local)
	}
	t := time.Date(1601, 1, 1, 0, 0, 0, 0, time.Local)
	d := time.Duration(epoch)
	for i := 0; i < 1000; i++ {
		t = t.Add(d)
	}
	return t
}
