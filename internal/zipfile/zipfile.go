// Package zipfile writes the zip archives HBD produces, export's --zip and the archive command's, and extracts them.
package zipfile

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/atomicfile"
)

// Write never leaves a partial archivePath. Names are slash-separated paths relative to root, each its own entry.
func Write(archivePath, root string, names []string) error {
	return atomicfile.Write(archivePath, func(out io.Writer) error {
		zipWriter := zip.NewWriter(out)
		for _, name := range names {
			if err := addFile(zipWriter, filepath.Join(root, filepath.FromSlash(name)), name); err != nil {
				return errors.Join(fmt.Errorf("add %s: %w", name, err), zipWriter.Close())
			}
		}
		if err := zipWriter.Close(); err != nil {
			return fmt.Errorf("finish %s: %w", archivePath, err)
		}
		return nil
	})
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

// Extract unpacks archivePath into dir, refusing any entry that would land outside it.
func Extract(archivePath, dir string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", archivePath, err)
	}
	defer zipReader.Close()

	for _, zipFile := range zipReader.File {
		name := filepath.FromSlash(zipFile.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("zip entry %q escapes destination", zipFile.Name)
		}
		target := filepath.Join(dir, name)
		if zipFile.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := extractFile(zipFile, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(zipFile *zip.File, target string) (err error) {
	entryReader, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer entryReader.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.Close()) }()

	for {
		_, err := io.CopyN(out, entryReader, 1<<20)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
