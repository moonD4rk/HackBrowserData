package safari

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/types"
)

type sourcePath struct {
	abs   string
	isDir bool
}

func file(abs string) sourcePath { return sourcePath{abs: abs} }

// buildSources dispatches between the default and named-profile path layouts.
//
// macOS 14+ layout:
//   - History, Cookie:  per-profile (separate files per profile UUID)
//   - Download:         shared plist, filtered by DownloadEntryProfileUUIDStringKey at extract time
//   - Bookmark:         shared plist, attributed to default only (no per-entry UUID available)
//   - Password:         macOS Keychain (shared, not listed)
func buildSources(p profileContext) map[types.Category][]sourcePath {
	if p.isDefault() {
		return defaultSources(p)
	}
	return namedSources(p)
}

// defaultSources: cookies try macOS 14+ container first, then the ≤13 legacy path.
func defaultSources(p profileContext) map[types.Category][]sourcePath {
	home := p.legacyHome
	containerCookies := filepath.Join(p.container, "Cookies", "Cookies.binarycookies")
	legacyCookies := filepath.Join(filepath.Dir(home), "Cookies", "Cookies.binarycookies")

	return map[types.Category][]sourcePath{
		types.History:  {file(filepath.Join(home, "History.db"))},
		types.Cookie:   {file(containerCookies), file(legacyCookies)},
		types.Bookmark: {file(filepath.Join(home, "Bookmarks.plist"))},
		types.Download: {file(filepath.Join(home, "Downloads.plist"))},
	}
}

// namedSources omits Bookmark (shared plist with no per-entry profile tag, so attributed to default).
// Download is included because Downloads.plist carries DownloadEntryProfileUUIDStringKey per entry;
// extractDownloads filters by owner UUID so default and named profiles each see their own downloads.
//
// LocalStorage slot for a follow-up PR:
//
//	file(filepath.Join(p.container, "WebKit/WebsiteDataStore", p.uuidLower, "LocalStorage"))
func namedSources(p profileContext) map[types.Category][]sourcePath {
	profileDir := filepath.Join(p.container, "Safari", "Profiles", p.uuidUpper)
	webkitStore := filepath.Join(p.container, "WebKit", "WebsiteDataStore", p.uuidLower)

	return map[types.Category][]sourcePath{
		types.History:  {file(filepath.Join(profileDir, "History.db"))},
		types.Cookie:   {file(filepath.Join(webkitStore, "Cookies", "Cookies.binarycookies"))},
		types.Download: {file(filepath.Join(p.legacyHome, "Downloads.plist"))},
	}
}
