package chromium

import (
	"path"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// ArchiveSource is one decryption-relevant file or directory plus its path inside the browser's
// User Data tree (forward-slash), so an archive can be re-expanded into a working profile layout.
type ArchiveSource struct {
	AbsPath   string
	LayoutRel string
	IsDir     bool
}

// installationFiles live at the User Data root (shared across profiles); archived for fidelity even
// though keys.json-based restore does not read them.
var installationFiles = []string{"Local State"}

// ArchiveSources lists the files an archive must capture for the given categories: the User Data root
// files (Local State), every resolved category source per profile, plus each profile's Preferences
// marker so a restore can rediscover the profile. LayoutRel is forward-slash, relative to the root.
func (b *Browser) ArchiveSources(categories []types.Category) []ArchiveSource {
	var out []ArchiveSource
	for _, name := range installationFiles {
		abs := filepath.Join(b.cfg.UserDataDir, name)
		if fileutil.FileExists(abs) {
			out = append(out, ArchiveSource{AbsPath: abs, LayoutRel: name, IsDir: false})
		}
	}
	for _, p := range b.profiles {
		// Flat-layout installs hold data directly under UserDataDir (profileDir == root); skip the
		// basename so the archive matches the real layout instead of inserting a phantom level.
		profileRel := ""
		if p.profileDir != b.cfg.UserDataDir {
			profileRel = filepath.Base(p.profileDir)
		}
		for _, marker := range profileMarkers {
			abs := filepath.Join(p.profileDir, marker)
			if fileutil.FileExists(abs) {
				out = append(out, ArchiveSource{
					AbsPath:   abs,
					LayoutRel: path.Join(profileRel, marker),
					IsDir:     false,
				})
			}
		}
		for _, cat := range categories {
			rp, ok := p.sourcePaths[cat]
			if !ok {
				continue
			}
			out = append(out, ArchiveSource{
				AbsPath:   rp.absPath,
				LayoutRel: path.Join(profileRel, rp.rel),
				IsDir:     rp.isDir,
			})
		}
	}
	return out
}
