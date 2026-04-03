package chromium

import (
	"os"
	"path/filepath"

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
	sources     map[types.Category][]sourcePath      // Category → candidate paths (priority order)
	extractors  map[types.Category]categoryExtractor // Category → custom extract function override
	sourcePaths map[types.Category]resolvedPath      // Category → discovered absolute path
}

// NewBrowsers discovers Chromium profiles under cfg.UserDataDir and returns
// one Browser per profile. Uses ReadDir to find profile directories,
// then Stat to check which data sources exist in each profile.
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

func (b *Browser) BrowserName() string { return b.cfg.Name }
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

	masterKey, err := b.getMasterKey(session)
	if err != nil {
		log.Debugf("get master key for %s: %v", b.BrowserName()+"/"+b.ProfileName(), err)
	}

	data := &types.BrowserData{}
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		b.extractCategory(data, cat, masterKey, path)
	}
	return data, nil
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

// getMasterKey retrieves the Chromium master encryption key.
//
// On Windows, the key is read from the Local State file and decrypted via DPAPI.
// On macOS, the key is derived from Keychain (Local State is not needed).
// On Linux, the key is derived from D-Bus Secret Service or a fallback password.
//
// The retriever is always called regardless of whether Local State exists,
// because macOS/Linux retrievers don't need it.
func (b *Browser) getMasterKey(session *filemanager.Session) ([]byte, error) {
	// Try to locate and copy Local State (needed on Windows, ignored on macOS/Linux).
	// Multi-profile layout: Local State is in the parent of profileDir.
	// Flat layout (Opera): Local State is alongside data files in profileDir.
	var localStateDst string
	for _, dir := range []string{filepath.Dir(b.profileDir), b.profileDir} {
		candidate := filepath.Join(dir, "Local State")
		if fileutil.IsFileExists(candidate) {
			localStateDst = filepath.Join(session.TempDir(), "Local State")
			if err := session.Acquire(candidate, localStateDst, false); err != nil {
				return nil, err
			}
			break
		}
	}

	retriever := keyretriever.DefaultRetriever(b.cfg.KeychainPassword)
	return retriever.RetrieveKey(b.cfg.Storage, localStateDst)
}

// extractCategory calls the appropriate extract function for a category.
// If a custom extractor is registered for this category (via extractorsForKind),
// it is used instead of the default switch logic.
func (b *Browser) extractCategory(data *types.BrowserData, cat types.Category, masterKey []byte, path string) {
	if ext, ok := b.extractors[cat]; ok {
		if err := ext.extract(masterKey, path, data); err != nil {
			log.Debugf("extract %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
		}
		return
	}

	var err error
	switch cat {
	case types.Password:
		data.Passwords, err = extractPasswords(masterKey, path)
	case types.Cookie:
		data.Cookies, err = extractCookies(masterKey, path)
	case types.History:
		data.Histories, err = extractHistories(path)
	case types.Download:
		data.Downloads, err = extractDownloads(path)
	case types.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case types.CreditCard:
		data.CreditCards, err = extractCreditCards(masterKey, path)
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

// discoverProfiles lists subdirectories of userDataDir that contain at least
// one known data source. Each such directory is a browser profile.
func discoverProfiles(userDataDir string, sources map[types.Category][]sourcePath) []string {
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		log.Debugf("read user data dir %s: %v", userDataDir, err)
		return nil
	}

	var profiles []string
	for _, e := range entries {
		if !e.IsDir() || isSkippedDir(e.Name()) {
			continue
		}
		dir := filepath.Join(userDataDir, e.Name())
		if hasAnySource(sources, dir) {
			profiles = append(profiles, dir)
		}
	}

	// Flat layout fallback (older Opera): data files directly in userDataDir
	if len(profiles) == 0 && hasAnySource(sources, userDataDir) {
		profiles = append(profiles, userDataDir)
	}
	return profiles
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
