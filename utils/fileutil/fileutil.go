package fileutil

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileExists checks if the file exists in the provided path.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// CompressDir compresses the directory into a zip file
func CompressDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir error: %w", err)
	}
	if len(files) == 0 {
		// Return an error if no files are found in the directory
		return fmt.Errorf("no files to compress in: %s", dir)
	}

	buffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buffer)
	defer func() {
		_ = zipWriter.Close()
	}()

	for _, file := range files {
		if err := addFileToZip(zipWriter, filepath.Join(dir, file.Name())); err != nil {
			return fmt.Errorf("failed to add file to zip: %w", err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("error closing zip writer: %w", err)
	}

	zipFilename := filepath.Join(dir, filepath.Base(dir)+".zip")
	return writeFile(buffer, zipFilename)
}

func addFileToZip(zw *zip.Writer, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", filename, err)
	}

	fw, err := zw.Create(filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("error creating zip entry for %s: %w", filename, err)
	}

	if _, err = fw.Write(content); err != nil {
		return fmt.Errorf("error writing content to zip for %s: %w", filename, err)
	}

	if err = os.Remove(filename); err != nil {
		return fmt.Errorf("error removing original file %s: %w", filename, err)
	}

	return nil
}

func writeFile(buffer *bytes.Buffer, filename string) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating output file %s: %w", filename, err)
	}
	defer func() {
		_ = outFile.Close()
	}()

	if _, err = buffer.WriteTo(outFile); err != nil {
		return fmt.Errorf("error writing data to file %s: %w", filename, err)
	}

	return nil
}

// ZipDir writes every file under srcDir into a new zip at zipPath, preserving the relative directory
// layout with forward-slash entry names. Unlike CompressDir it neither flattens names nor deletes the
// source — it is the producer side of cross-host archive transport.
func ZipDir(zipPath, srcDir string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", zipPath, err)
	}
	defer func() { _ = out.Close() }()

	zw := zip.NewWriter(out)
	walkErr := filepath.WalkDir(srcDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(p) //nolint:gosec // G122: staging tree is created and populated by us
		if err != nil {
			return err
		}
		defer func() { _ = src.Close() }()
		_, err = io.Copy(w, src)
		return err
	})
	if walkErr != nil {
		_ = zw.Close()
		return fmt.Errorf("zip %s: %w", srcDir, walkErr)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip %s: %w", zipPath, err)
	}
	return nil
}

// Unzip extracts zipPath into destDir, rejecting any entry whose path would escape destDir (Zip-Slip)
// since a transported archive is not fully trusted.
func Unzip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() { _ = r.Close() }()

	root := filepath.Clean(destDir)
	for _, f := range r.File {
		target := filepath.Join(root, filepath.FromSlash(f.Name))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q escapes destination", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	for {
		_, err := io.CopyN(out, rc, 1<<20)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
