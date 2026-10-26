package safari

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/category"
)

type sourcePath struct {
	abs   string
	isDir bool
}

func file(abs string) sourcePath { return sourcePath{abs: abs} }
func dir(abs string) sourcePath  { return sourcePath{abs: abs, isDir: true} }

// buildSources dispatches between the default and named-profile path layouts.
//
// macOS 14+ layout:
//   - History, Cookie:  per-profile (separate files per profile UUID)
//   - Download:         shared plist, filtered by DownloadEntryProfileUUIDStringKey at extract time
//   - Bookmark:         shared plist, attributed to default only (no per-entry UUID available)
//   - Password:         macOS Keychain (shared, not listed)
func buildSources(p profileContext) map[category.Category][]sourcePath {
	if p.isDefault() {
		return defaultSources(p)
	}
	return namedSources(p)
}

// defaultSources: cookies try macOS 14+ container first, then the ≤13 legacy path.
// LocalStorage for the default profile lives under WebsiteData/Default — the pre-profile-era
// WebKit store that stays readable even after profiles are introduced.
func defaultSources(p profileContext) map[category.Category][]sourcePath {
	home := p.legacyHome
	containerCookies := filepath.Join(p.container, "Cookies", "Cookies.binarycookies")
	legacyCookies := filepath.Join(filepath.Dir(home), "Cookies", "Cookies.binarycookies")
	defaultLocalStorage := filepath.Join(p.container, "WebKit", "WebsiteData", "Default")

	return map[category.Category][]sourcePath{
		category.History:      {file(filepath.Join(home, "History.db"))},
		category.Cookie:       {file(containerCookies), file(legacyCookies)},
		category.Bookmark:     {file(filepath.Join(home, "Bookmarks.plist"))},
		category.Download:     {file(filepath.Join(home, "Downloads.plist"))},
		category.LocalStorage: {dir(defaultLocalStorage)},
	}
}

// namedSources omits Bookmark (shared plist with no per-entry profile tag, so attributed to default).
// Download is included because Downloads.plist carries DownloadEntryProfileUUIDStringKey per entry;
// extractDownloads filters by owner UUID so default and named profiles each see their own downloads.
// LocalStorage lives under WebKit/WebsiteDataStore/<uuidLower>/Origins — Safari 17+ uses a nested
// <top-frame-hash>/<frame-hash>/LocalStorage/localstorage.sqlite3 layout; the flat
// WebsiteDataStore/<uuid>/LocalStorage directory from older builds is empty on modern Safari.
func namedSources(p profileContext) map[category.Category][]sourcePath {
	profileDir := filepath.Join(p.container, "Safari", "Profiles", p.uuidUpper)
	webkitStore := filepath.Join(p.container, "WebKit", "WebsiteDataStore", p.uuidLower)

	return map[category.Category][]sourcePath{
		category.History:      {file(filepath.Join(profileDir, "History.db"))},
		category.Cookie:       {file(filepath.Join(webkitStore, "Cookies", "Cookies.binarycookies"))},
		category.Download:     {file(filepath.Join(p.legacyHome, "Downloads.plist"))},
		category.LocalStorage: {dir(filepath.Join(webkitStore, "Origins"))},
	}
}
