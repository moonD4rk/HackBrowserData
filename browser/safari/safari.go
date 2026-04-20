package safari

import (
	"os"
	"path/filepath"
	"time"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser is one Safari profile's data ready for extraction. Passwords come from the shared macOS
// Keychain; everything else reads from the profile's directories.
type Browser struct {
	cfg              types.BrowserConfig
	profile          profileContext
	keychainPassword string
	sourcePaths      map[types.Category]resolvedPath
}

func (b *Browser) SetKeychainPassword(password string) { b.keychainPassword = password }

// NewBrowsers returns one Browser per Safari profile with resolvable data. Named profiles are
// enumerated from SafariTabs.db.
func NewBrowsers(cfg types.BrowserConfig) ([]*Browser, error) {
	var browsers []*Browser
	for _, p := range discoverSafariProfiles(cfg.UserDataDir) {
		paths := resolveProfilePaths(p)
		if len(paths) == 0 {
			continue
		}
		browsers = append(browsers, &Browser{
			cfg:         cfg,
			profile:     p,
			sourcePaths: paths,
		})
	}
	return browsers, nil
}

func resolveProfilePaths(p profileContext) map[types.Category]resolvedPath {
	return resolveSourcePaths(buildSources(p))
}

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) ProfileName() string { return b.profile.name }

func (b *Browser) ProfileDir() string {
	if b.profile.isDefault() {
		return b.profile.legacyHome
	}
	return filepath.Join(b.profile.container, "Safari", "Profiles", b.profile.uuidUpper)
}

func (b *Browser) Extract(categories []types.Category) (*types.BrowserData, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	data := &types.BrowserData{}
	for _, cat := range categories {
		// Keychain is user-scope, not per-profile — attribute only to default to avoid duplicates.
		if cat == types.Password {
			if b.profile.isDefault() {
				b.extractCategory(data, cat, "")
			}
			continue
		}
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		b.extractCategory(data, cat, path)
	}
	return data, nil
}

func (b *Browser) CountEntries(categories []types.Category) (map[types.Category]int, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	counts := make(map[types.Category]int)
	for _, cat := range categories {
		if cat == types.Password {
			if b.profile.isDefault() {
				counts[cat] = b.countCategory(cat, "")
			}
			continue
		}
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		counts[cat] = b.countCategory(cat, path)
	}
	return counts, nil
}

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

func (b *Browser) extractCategory(data *types.BrowserData, cat types.Category, path string) {
	var err error
	switch cat {
	case types.Password:
		data.Passwords, err = extractPasswords(b.keychainPassword)
	case types.History:
		data.Histories, err = extractHistories(path)
	case types.Cookie:
		data.Cookies, err = extractCookies(path)
	case types.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case types.Download:
		data.Downloads, err = extractDownloads(path, b.profile.downloadOwnerUUID())
	default:
		return
	}
	if err != nil {
		log.Debugf("extract %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
}

func (b *Browser) countCategory(cat types.Category, path string) int {
	var count int
	var err error
	switch cat {
	case types.Password:
		count, err = countPasswords(b.keychainPassword)
	case types.History:
		count, err = countHistories(path)
	case types.Cookie:
		count, err = countCookies(path)
	case types.Bookmark:
		count, err = countBookmarks(path)
	case types.Download:
		count, err = countDownloads(path, b.profile.downloadOwnerUUID())
	default:
		// Unsupported categories silently return 0.
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
	return count
}

type resolvedPath struct {
	absPath string
	isDir   bool
}

// resolveSourcePaths returns only paths that exist; first matching candidate wins per category.
func resolveSourcePaths(sources map[types.Category][]sourcePath) map[types.Category]resolvedPath {
	resolved := make(map[types.Category]resolvedPath)
	for cat, candidates := range sources {
		for _, sp := range candidates {
			info, err := os.Stat(sp.abs)
			if err != nil {
				continue
			}
			if sp.isDir == info.IsDir() {
				resolved[cat] = resolvedPath{sp.abs, sp.isDir}
				break
			}
		}
	}
	return resolved
}

// Safari's History.db uses the Core Data epoch (2001-01-01) instead of Unix epoch.
const coreDataEpochOffset = 978307200

func coredataTimestamp(seconds float64) time.Time {
	return time.Unix(int64(seconds)+coreDataEpochOffset, 0)
}
