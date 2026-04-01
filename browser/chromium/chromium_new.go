package chromium

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// Browser represents a single Chromium profile ready for extraction.
type Browser struct {
	cfg         types.BrowserConfig
	name        string                        // display name: "Chrome-Default"
	profileDir  string                        // absolute path to profile directory
	sources     map[types.Category]dataSource // Category → file source mapping
	queries     map[types.Category]string     // Category → SQL query override (Yandex)
	sourcePaths map[types.Category]string     // Category → absolute source file path (populated by Walk)
}

// NewBrowsers discovers Chromium profiles under userDataDir and returns
// one Browser per profile. Profile discovery uses filepath.Walk
// for maximum compatibility with older Chromium versions.
func NewBrowsers(cfg types.BrowserConfig, userDataDir string) ([]*Browser, error) {
	sources := sourcesForKind(cfg.Kind)
	queries := queriesForKind(cfg.Kind)
	fileNames := sourceFileNames(sources)

	profiles := discoverProfiles(userDataDir, fileNames)
	if len(profiles) == 0 {
		return nil, nil
	}

	browsers := make([]*Browser, 0, len(profiles))
	for profileDir, filePaths := range profiles {
		b := &Browser{
			cfg:         cfg,
			name:        cfg.Name + "-" + filepath.Base(profileDir),
			profileDir:  profileDir,
			sources:     sources,
			queries:     queries,
			sourcePaths: resolveSourcePaths(sources, filePaths),
		}
		browsers = append(browsers, b)
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

	// Acquire files: copy from profile to temp
	tempPaths := b.acquireFiles(session, categories)

	// Get master key (platform-specific)
	masterKey, err := b.getMasterKey(session)
	if err != nil {
		log.Debugf("get master key for %s: %v", b.name, err)
		// Continue without master key — non-encrypted categories still work
	}

	// Extract each category
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
		srcPath, ok := b.sourcePaths[cat]
		if !ok {
			continue
		}
		src := b.sources[cat]
		dst := filepath.Join(session.TempDir(), cat.String())
		if err := session.Acquire(srcPath, dst, src.isDir); err != nil {
			log.Debugf("acquire %s: %v", cat, err)
			continue
		}
		tempPaths[cat] = dst
	}
	return tempPaths
}

// getMasterKey retrieves the Chromium master encryption key.
func (b *Browser) getMasterKey(session *filemanager.Session) ([]byte, error) {
	// Local State is one level above the profile directory
	localStateSrc := filepath.Join(filepath.Dir(b.profileDir), "Local State")
	if !fileutil.IsFileExists(localStateSrc) {
		return nil, nil // old Chromium without Local State — no encrypted data
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
// Profile discovery
// ---------------------------------------------------------------------------

// discoverProfiles walks userDataDir and groups discovered data files by profile.
// Returns map[profileDir]map[fileName]absolutePath.
func discoverProfiles(userDataDir string, fileNames map[string]bool) map[string]map[string]string {
	profiles := make(map[string]map[string]string)

	_ = filepath.Walk(userDataDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}

		name := info.Name()
		if !fileNames[name] {
			return nil
		}

		// Skip known non-profile directories
		if containsAny(path, "System Profile", "Snapshot", "Guest Profile") {
			return nil
		}

		profileDir := profileDirFromPath(path, name)

		if profiles[profileDir] == nil {
			profiles[profileDir] = make(map[string]string)
		}
		profiles[profileDir][name] = path
		return nil
	})

	return profiles
}

// profileDirFromPath extracts the profile directory from a file path.
// For "Network/Cookies" the profile is two levels up; for other files one level up.
func profileDirFromPath(path, fileName string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "Network" && fileName == "Cookies" {
		dir = filepath.Dir(dir)
	}
	return dir
}

// containsAny returns true if path contains any of the substrings.
func containsAny(path string, subs ...string) bool {
	for _, s := range subs {
		if strings.Contains(path, s) {
			return true
		}
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

// sourceFileNames collects all unique file names from the source mapping.
func sourceFileNames(sources map[types.Category]dataSource) map[string]bool {
	names := make(map[string]bool)
	for _, src := range sources {
		for _, p := range src.paths {
			names[filepath.Base(p)] = true
		}
	}
	return names
}

// resolveSourcePaths maps each Category to its actual source file path
// using the discovered file paths from Walk.
func resolveSourcePaths(sources map[types.Category]dataSource, filePaths map[string]string) map[types.Category]string {
	resolved := make(map[types.Category]string)
	for cat, src := range sources {
		for _, rel := range src.paths {
			fileName := filepath.Base(rel)
			if absPath, ok := filePaths[fileName]; ok {
				resolved[cat] = absPath
				break
			}
		}
	}
	return resolved
}
