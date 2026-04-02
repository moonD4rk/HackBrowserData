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

// firefoxSources defines the Firefox file layout.
// Each category maps to one or more candidate paths tried in priority order;
// the first existing path wins.
// Firefox does not support SessionStorage or CreditCard extraction.
var firefoxSources = map[types.Category][]sourcePath{
	types.Password:     {file("logins.json")},
	types.Cookie:       {file("cookies.sqlite")},
	types.History:      {file("places.sqlite")},
	types.Download:     {file("places.sqlite")},
	types.Bookmark:     {file("places.sqlite")},
	types.Extension:    {file("extensions.json")},
	types.LocalStorage: {file("webappsstore.sqlite")},
}
