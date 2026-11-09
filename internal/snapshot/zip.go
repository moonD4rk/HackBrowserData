package snapshot

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/zipfile"
)

func (d *Dir) Zip(archivePath string) error {
	var names []string
	walkErr := filepath.WalkDir(d.path, func(walkedPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(d.path, walkedPath)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(relativePath))
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("zip %s: %w", d.path, walkErr)
	}
	return zipfile.Write(archivePath, d.path, names)
}
