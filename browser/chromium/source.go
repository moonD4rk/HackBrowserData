package chromium

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/types"
)

// sourcePath describes a single candidate location for browser data,
// relative to the profile directory.
type sourcePath struct {
	rel   string // relative path from profileDir, e.g. "Network/Cookies"
	isDir bool   // true for directory targets (LevelDB, Session Storage)
}

func file(rel string) sourcePath { return sourcePath{rel: filepath.FromSlash(rel), isDir: false} }
func dir(rel string) sourcePath  { return sourcePath{rel: filepath.FromSlash(rel), isDir: true} }

// dataSource holds one or more candidate sourcePaths in priority order.
// The first candidate that exists on disk wins.
type dataSource struct {
	candidates []sourcePath
}

// chromiumSources defines the standard Chromium file layout.
var chromiumSources = map[types.Category]dataSource{
	types.Password:       {candidates: []sourcePath{file("Login Data")}},
	types.Cookie:         {candidates: []sourcePath{file("Network/Cookies"), file("Cookies")}},
	types.History:        {candidates: []sourcePath{file("History")}},
	types.Download:       {candidates: []sourcePath{file("History")}},
	types.Bookmark:       {candidates: []sourcePath{file("Bookmarks")}},
	types.CreditCard:     {candidates: []sourcePath{file("Web Data")}},
	types.Extension:      {candidates: []sourcePath{file("Secure Preferences")}},
	types.LocalStorage:   {candidates: []sourcePath{dir("Local Storage/leveldb")}},
	types.SessionStorage: {candidates: []sourcePath{dir("Session Storage")}},
}

// yandexSourceOverrides contains only the entries that differ from chromiumSources.
var yandexSourceOverrides = map[types.Category]dataSource{
	types.Password:   {candidates: []sourcePath{file("Ya Passman Data")}},
	types.CreditCard: {candidates: []sourcePath{file("Ya Credit Cards")}},
}

// yandexSources returns chromiumSources with Yandex-specific overrides applied.
func yandexSources() map[types.Category]dataSource {
	sources := make(map[types.Category]dataSource, len(chromiumSources))
	for k, v := range chromiumSources {
		sources[k] = v
	}
	for k, v := range yandexSourceOverrides {
		sources[k] = v
	}
	return sources
}

// yandexQueryOverrides provides SQL query overrides for Yandex Browser.
var yandexQueryOverrides = map[types.Category]string{
	types.Password: `SELECT action_url, username_value, password_value, date_created FROM logins`,
}
