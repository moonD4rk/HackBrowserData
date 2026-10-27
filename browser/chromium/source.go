package chromium

import (
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	ichromium "github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

const accountLoginData = `Login Data For Account`

// sourcePath describes a single candidate location for browser data,
// relative to the profile directory.
type sourcePath struct {
	rel   string // relative path from profileDir, e.g. "Network/Cookies"
	isDir bool   // true for directory targets (LevelDB, Session Storage)
}

// rel stays slash-canonical (e.g. "Network/Cookies"); filepath.Join converts at resolve time, and
// archive reuses it verbatim as a forward-slash zip entry name.
func file(rel string) sourcePath { return sourcePath{rel: rel, isDir: false} }
func dir(rel string) sourcePath  { return sourcePath{rel: rel, isDir: true} }

// chromiumSources defines the standard Chromium file layout.
// Each category maps to one or more candidate paths tried in priority order;
// the first existing path wins.
var chromiumSources = map[category.Category][]sourcePath{
	category.Password:       {file("Login Data"), file(accountLoginData)},
	category.Cookie:         {file("Network/Cookies"), file("Cookies")},
	category.History:        {file("History")},
	category.Download:       {file("History")},
	category.Bookmark:       {file("Bookmarks")},
	category.CreditCard:     {file("Web Data")},
	category.Extension:      {file("Secure Preferences")},
	category.LocalStorage:   {dir("Local Storage/leveldb")},
	category.SessionStorage: {dir("Session Storage")},
}

// sourcesForKind returns the source mapping for a browser kind.
func sourcesForKind(kind types.BrowserKind) map[category.Category][]sourcePath {
	switch kind {
	case types.ChromiumYandex:
		return yandexSources()
	default:
		return chromiumSources
	}
}

// yandexSourceOverrides contains only the entries that differ from chromiumSources.
var yandexSourceOverrides = map[category.Category][]sourcePath{
	category.Password:   {file("Ya Passman Data")},
	category.CreditCard: {file("Ya Credit Cards")},
}

// yandexSources returns chromiumSources with Yandex-specific overrides applied.
func yandexSources() map[category.Category][]sourcePath {
	sources := make(map[category.Category][]sourcePath, len(chromiumSources))
	for k, v := range chromiumSources {
		sources[k] = v
	}
	for k, v := range yandexSourceOverrides {
		sources[k] = v
	}
	return sources
}

// extractorsFor picks the Chromium base's per-fork extractors by kind, which is what chose them before the base had
// its own override table.
func extractorsFor(kind types.BrowserKind) ibrowser.Extractors {
	var id string
	switch kind {
	case types.ChromiumYandex:
		id = "yandex"
	case types.ChromiumOpera:
		id = "opera"
	default:
		id = "chromium"
	}
	return ichromium.Extractors(ibrowser.UserDataDir{Def: ibrowser.Def{ID: id, Base: ibrowser.Chromium}}, ibrowser.Profile{})
}
