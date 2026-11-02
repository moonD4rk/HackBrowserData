// Package zipfile writes the zip archives HBD produces: export's --zip and the archive command's.
package zipfile

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Write renames a finished temp file into place, so archivePath is never a partial zip and a failure keeps the old one.
// Names are slash-separated paths relative to root, and each becomes the entry of that name.
func Write(archivePath, root string, names []string) (err error) {
	temporary, err := os.OpenFile(archivePath+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", archivePath, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(temporary.Name()))
		}
	}()
	zipWriter := zip.NewWriter(temporary)
	for _, name := range names {
		if err := addFile(zipWriter, filepath.Join(root, filepath.FromSlash(name)), name); err != nil {
			return errors.Join(fmt.Errorf("add %s: %w", name, err), zipWriter.Close(), temporary.Close())
		}
	}
	if err := zipWriter.Close(); err != nil {
		return errors.Join(fmt.Errorf("finish %s: %w", archivePath, err), temporary.Close())
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("finish %s: %w", archivePath, err)
	}
	if err := os.Rename(temporary.Name(), archivePath); err != nil {
		return fmt.Errorf("move zip into place: %w", err)
	}
	return nil
}

func addFile(zipWriter *zip.Writer, path, name string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	entryWriter, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(entryWriter, file)
	return err
}
