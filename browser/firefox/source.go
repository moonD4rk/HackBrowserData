package firefox

import "github.com/moond4rk/hackbrowserdata/types"

// dataSource maps a Category to one or more candidate file paths within a profile directory.
type dataSource struct {
	paths []string // candidate relative paths in priority order
	isDir bool     // true for directories (unused in Firefox, all sources are files)
}

// firefoxSources defines the Firefox file layout.
// Firefox does not support SessionStorage or CreditCard extraction.
var firefoxSources = map[types.Category]dataSource{
	types.Password:     {paths: []string{"logins.json"}},
	types.Cookie:       {paths: []string{"cookies.sqlite"}},
	types.History:      {paths: []string{"places.sqlite"}},
	types.Download:     {paths: []string{"places.sqlite"}}, // same file as History
	types.Bookmark:     {paths: []string{"places.sqlite"}}, // same file as History
	types.Extension:    {paths: []string{"extensions.json"}},
	types.LocalStorage: {paths: []string{"webappsstore.sqlite"}},
}
