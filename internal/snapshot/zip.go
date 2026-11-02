package snapshot

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

func unzip(archivePath, destDir string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", archivePath, err)
	}
	defer zipReader.Close()

	root := filepath.Clean(destDir)
	for _, zipFile := range zipReader.File {
		target := filepath.Join(root, filepath.FromSlash(zipFile.Name))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q escapes destination", zipFile.Name)
		}
		if zipFile.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := writeZipEntry(zipFile, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(zipFile *zip.File, target string) (err error) {
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
