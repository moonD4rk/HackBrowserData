package snapshot

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/zipfile"
)

func (r *Root) Zip(archivePath string) error {
	var names []string
	walkErr := filepath.WalkDir(r.path, func(walkedPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(r.path, walkedPath)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(relativePath))
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("zip %s: %w", r.path, walkErr)
	}
	return zipfile.Write(archivePath, r.path, names)
}
