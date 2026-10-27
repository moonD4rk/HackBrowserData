package browser

import (
	"context"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Archivable is implemented by installations that can enumerate their decryption-relevant files for
// cross-host transport (Chromium only).
type Archivable interface {
	BrowserKey() string
	UserDataDir() string
	ArchiveSources(categories []category.Category) []chromium.ArchiveSource
}

// WriteArchive packs each browser's decryption-relevant files into a zip whose internal layout is
// <browser-key>/<User Data layout>, so a restore can re-expand it and decrypt with a keys.json. Returns
// the number of source entries captured (a directory source counts once).
func WriteArchive(browsers []Browser, categories []category.Category, outPath string) (int, error) {
	files := archiveFiles(browsers, categories)

	d, err := snapshot.Temp()
	if err != nil {
		return 0, err
	}
	defer closeSnapshot(d)

	if err := d.Capture(context.Background(), files); err != nil {
		return 0, err
	}
	count := 0
	for _, f := range files {
		if _, err := d.Snapshot(f.DirID).Path(f.Rel); err != nil {
			log.Warnf("archive: acquire %s/%s: %v", f.DirID, f.Rel, err)
			continue
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("no decryption-relevant files found to archive")
	}
	if err := d.Zip(outPath); err != nil {
		return 0, fmt.Errorf("write archive %s: %w", outPath, err)
	}
	return count, nil
}

// archiveFiles lists each source once; the browser key is its snapshot ID, so the zip's top level stays <browser-key>/.
func archiveFiles(browsers []Browser, categories []category.Category) []snapshot.File {
	var files []snapshot.File
	seen := make(map[string]bool)
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
			files = append(files, snapshot.File{DirID: key, From: archivable.UserDataDir(), Rel: src.LayoutRel, IsDir: src.IsDir})
		}
	}
	return files
}

func closeSnapshot(d *snapshot.Dir) {
	if err := d.Close(); err != nil {
		log.Warnf("archive: %v", err)
	}
}
