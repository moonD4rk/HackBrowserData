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

// resolvedPath holds the absolute path and type for a discovered source.
type resolvedPath struct {
	absPath string
	isDir   bool
}

// Browser represents a single Chromium profile ready for extraction.
type Browser struct {
	cfg         types.BrowserConfig
	name        string                          // display name: "Chrome-Default"
	profileDir  string                          // absolute path to profile directory
	sources     map[types.Category]dataSource   // Category → source mapping
	queries     map[types.Category]string       // Category → SQL query override (Yandex)
	sourcePaths map[types.Category]resolvedPath // Category → discovered absolute path
}

// NewBrowsers discovers Chromium profiles under userDataDir and returns
// one Browser per profile. Uses ReadDir to find profile directories,
// then Stat to check which data sources exist in each profile.
func NewBrowsers(cfg types.BrowserConfig, userDataDir string) ([]*Browser, error) {
	sources := sourcesForKind(cfg.Kind)
	queries := queriesForKind(cfg.Kind)

	profileDirs := discoverProfiles(userDataDir, sources)
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
			name:        cfg.Name + "-" + filepath.Base(profileDir),
			profileDir:  profileDir,
			sources:     sources,
			queries:     queries,
			sourcePaths: sourcePaths,
		})
	}
	return browsers, nil
}

func (b *Browser) Name() string {
	return b.name
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
		log.Debugf("get master key for %s: %v", b.name, err)
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
func (b *Browser) getMasterKey(session *filemanager.Session) ([]byte, error) {
	localStateSrc := filepath.Join(filepath.Dir(b.profileDir), "Local State")
	if !fileutil.IsFileExists(localStateSrc) {
		return nil, nil
	}

	localStateDst := filepath.Join(session.TempDir(), "Local State")
	if err := session.Acquire(localStateSrc, localStateDst, false); err != nil {
		return nil, err
	}

	retriever := keyretriever.DefaultRetriever("")
	return retriever.RetrieveKey(b.cfg.Storage, localStateDst)
}

// extractCategory calls the appropriate extract function for a category.
func (b *Browser) extractCategory(data *types.BrowserData, cat types.Category, masterKey []byte, path string) {
	var err error
	switch cat {
	case types.Password:
		query := defaultLoginQuery
		if q, ok := b.queries[types.Password]; ok {
			query = q
		}
		data.Passwords, err = extractPasswords(masterKey, path, query)
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
		log.Debugf("extract %s for %s: %v", cat, b.name, err)
	}
}

// ---------------------------------------------------------------------------
// Profile discovery (ReadDir + Stat)
// ---------------------------------------------------------------------------

// discoverProfiles lists subdirectories of userDataDir that contain at least
// one known data source. Each such directory is a browser profile.
func discoverProfiles(userDataDir string, sources map[types.Category]dataSource) []string {
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
func hasAnySource(sources map[types.Category]dataSource, dir string) bool {
	for _, ds := range sources {
		for _, sp := range ds.candidates {
			abs := filepath.Join(dir, sp.rel)
			if _, err := os.Stat(abs); err == nil {
				return true
			}
		}
	}
	return false
}

// resolveSourcePaths checks which sources actually exist in profileDir.
// Candidates are tried in priority order; the first existing path wins.
func resolveSourcePaths(sources map[types.Category]dataSource, profileDir string) map[types.Category]resolvedPath {
	resolved := make(map[types.Category]resolvedPath)
	for cat, ds := range sources {
		for _, sp := range ds.candidates {
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

// ---------------------------------------------------------------------------
// Source helpers
// ---------------------------------------------------------------------------

// sourcesForKind returns the source mapping for a browser kind.
func sourcesForKind(kind types.BrowserKind) map[types.Category]dataSource {
	if kind == types.KindChromiumYandex {
		return yandexSources()
	}
	return chromiumSources
}

// queriesForKind returns SQL query overrides for a browser kind.
func queriesForKind(kind types.BrowserKind) map[types.Category]string {
	if kind == types.KindChromiumYandex {
		return yandexQueryOverrides
	}
	return nil
}
