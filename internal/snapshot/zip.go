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
)

// Zip writes to archivePath through a temp file beside it, so a failed archive leaves no partial file.
func (d *Dir) Zip(archivePath string) (err error) {
	target, existing, err := zipTarget(archivePath)
	if err != nil {
		return err
	}
	temporary, err := createBeside(target)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(temporary.Name())
		}
	}()

	if err := writeZip(temporary, d.path); err != nil {
		return errors.Join(err, temporary.Close())
	}
	if existing != nil {
		if err := temporary.Chmod(existing.Mode().Perm()); err != nil {
			return errors.Join(fmt.Errorf("keep the mode of %s: %w", archivePath, err), temporary.Close())
		}
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close zip %s: %w", temporary.Name(), err)
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		return fmt.Errorf("move zip into place: %w", err)
	}
	return nil
}

// zipTarget follows a symlinked archivePath, so the replaced archive keeps the link and the mode os.Create would keep.
func zipTarget(archivePath string) (string, fs.FileInfo, error) {
	target := archivePath
	if resolved, err := filepath.EvalSymlinks(archivePath); err == nil {
		target = resolved
	}
	info, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return target, nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("create %s: %w", archivePath, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("create %s: is a directory", archivePath)
	}
	return target, info, nil
}

// createBeside keeps os.Create's umask-governed mode for a new archive, which os.CreateTemp would force to 0600.
func createBeside(target string) (*os.File, error) {
	dir, base := filepath.Split(target)
	for i := 0; i < 100; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%s.%d-%d.tmp", base, os.Getpid(), i))
		file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", target, err)
		}
		return file, nil
	}
	return nil, fmt.Errorf("create %s: no free temp name beside it", target)
}

func writeZip(writer io.Writer, root string) error {
	zipWriter := zip.NewWriter(writer)
	walkErr := filepath.WalkDir(root, func(walkedPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(root, walkedPath)
		if err != nil {
			return err
		}
		return addZipEntry(zipWriter, walkedPath, filepath.ToSlash(relativePath))
	})
	if walkErr != nil {
		return errors.Join(fmt.Errorf("zip %s: %w", root, walkErr), zipWriter.Close())
	}
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	return nil
}

func addZipEntry(zipWriter *zip.Writer, path, name string) error {
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
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
