// Package atomicfile writes a file in full or not at all, so no reader ever sees a partial one.
package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Write renames a finished temp file into place, so path is never partial and a failure keeps the old file.
func Write(path string, write func(io.Writer) error) (err error) {
	// CreateTemp makes a new 0600 file under an unpredictable name, so a planted file or symlink is never written through.
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(temporary.Name()))
		}
	}()
	if err := write(temporary); err != nil {
		return errors.Join(err, temporary.Close())
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("finish %s: %w", path, err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("move %s into place: %w", path, err)
	}
	return nil
}
