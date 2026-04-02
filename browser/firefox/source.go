package firefox

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/types"
)

// sourcePath describes a single candidate location for browser data,
// relative to the profile directory.
type sourcePath struct {
	rel   string // relative path from profileDir
	isDir bool   // true for directory targets
}

func file(rel string) sourcePath { return sourcePath{rel: filepath.FromSlash(rel), isDir: false} }

// dataSource holds one or more candidate sourcePaths in priority order.
type dataSource struct {
	candidates []sourcePath
}

// firefoxSources defines the Firefox file layout.
// Firefox does not support SessionStorage or CreditCard extraction.
var firefoxSources = map[types.Category]dataSource{
	types.Password:     {candidates: []sourcePath{file("logins.json")}},
	types.Cookie:       {candidates: []sourcePath{file("cookies.sqlite")}},
	types.History:      {candidates: []sourcePath{file("places.sqlite")}},
	types.Download:     {candidates: []sourcePath{file("places.sqlite")}},
	types.Bookmark:     {candidates: []sourcePath{file("places.sqlite")}},
	types.Extension:    {candidates: []sourcePath{file("extensions.json")}},
	types.LocalStorage: {candidates: []sourcePath{file("webappsstore.sqlite")}},
}
