package browser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// Archivable is implemented by installations that can enumerate their decryption-relevant files for
// cross-host transport (Chromium only).
type Archivable interface {
	BrowserKey() string
	ArchiveSources(categories []types.Category) []chromium.ArchiveSource
}

// WriteArchive packs each browser's decryption-relevant files into a zip whose internal layout is
// <browser-key>/<User Data layout>, so a restore can re-expand it and decrypt with a keys.json. Files
// are staged through a locked-file session first because Windows holds exclusive SQLite locks. Returns
// the number of source entries staged (a directory source counts once).
func WriteArchive(browsers []Browser, categories []types.Category, outPath string) (int, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return 0, err
	}
	defer session.Cleanup()

	staging := session.TempDir()
	seen := make(map[string]bool)
	count := 0
	for _, b := range browsers {
		archivable, ok := b.(Archivable)
		if !ok {
			continue
		}
		key := archivable.BrowserKey()
		for _, src := range archivable.ArchiveSources(categories) {
			entry := key + "/" + src.LayoutRel
			if seen[entry] {
				continue
			}
			seen[entry] = true

			dst := filepath.Join(staging, key, filepath.FromSlash(src.LayoutRel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				log.Warnf("archive: %s: %v", entry, err)
				continue
			}
			if err := session.Acquire(src.AbsPath, dst, src.IsDir); err != nil {
				log.Warnf("archive: acquire %s: %v", entry, err)
				continue
			}
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("no decryption-relevant files found to archive")
	}
	if err := fileutil.ZipDir(outPath, staging); err != nil {
		return 0, fmt.Errorf("write archive %s: %w", outPath, err)
	}
	return count, nil
}
