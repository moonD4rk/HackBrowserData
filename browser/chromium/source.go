package chromium

import "github.com/moond4rk/hackbrowserdata/types"

// dataSource maps a Category to one or more candidate file paths within a profile directory.
// paths are tried in order; the first one that exists is used.
type dataSource struct {
	paths []string // candidate relative paths in priority order
	isDir bool     // true for LevelDB directories
}

// chromiumSources defines the standard Chromium file layout.
var chromiumSources = map[types.Category]dataSource{
	types.Password:       {paths: []string{"Login Data"}},
	types.Cookie:         {paths: []string{"Network/Cookies", "Cookies"}},
	types.History:        {paths: []string{"History"}},
	types.Download:       {paths: []string{"History"}}, // same file, different query
	types.Bookmark:       {paths: []string{"Bookmarks"}},
	types.CreditCard:     {paths: []string{"Web Data"}},
	types.Extension:      {paths: []string{"Secure Preferences"}},
	types.LocalStorage:   {paths: []string{"Local Storage/leveldb"}, isDir: true},
	types.SessionStorage: {paths: []string{"Session Storage"}, isDir: true},
}

// yandexSourceOverrides contains only the entries that differ from chromiumSources.
// At initialization time, these are merged into a copy of chromiumSources.
var yandexSourceOverrides = map[types.Category]dataSource{
	types.Password:   {paths: []string{"Ya Passman Data"}},
	types.CreditCard: {paths: []string{"Ya Credit Cards"}},
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
// Yandex uses action_url instead of origin_url for password storage.
var yandexQueryOverrides = map[types.Category]string{
	types.Password: `SELECT action_url, username_value, password_value, date_created FROM logins`,
}
