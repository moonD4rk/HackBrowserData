package firefox

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/category"
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
var firefoxSources = map[category.Category][]sourcePath{
	category.Password:     {file("logins.json")},
	category.Cookie:       {file("cookies.sqlite")},
	category.History:      {file("places.sqlite")},
	category.Download:     {file("places.sqlite")},
	category.Bookmark:     {file("places.sqlite")},
	category.Extension:    {file("extensions.json")},
	category.LocalStorage: {file("webappsstore.sqlite")},
}
