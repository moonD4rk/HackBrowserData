package safari

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/types"
)

// sourcePath describes a single candidate location for browser data,
// relative to the Safari data directory.
type sourcePath struct {
	rel   string // relative path from dataDir
	isDir bool   // true for directory targets
}

func file(rel string) sourcePath { return sourcePath{rel: filepath.FromSlash(rel), isDir: false} }

// safariSources defines the Safari file layout.
// Each category maps to one or more candidate paths tried in priority order;
// the first existing path wins.
var safariSources = map[types.Category][]sourcePath{
	types.History: {file("History.db")},
}
