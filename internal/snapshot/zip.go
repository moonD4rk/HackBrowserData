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

// Zip writes the snapshot directory to dst through a temp file beside it, so a failed archive leaves no partial file.
func (d *Dir) Zip(dst string) (err error) {
	target, existing, err := zipTarget(dst)
	if err != nil {
		return err
	}
	tmp, err := createBeside(target)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()

	if err := writeZip(tmp, d.path); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if existing != nil {
		if err := tmp.Chmod(existing.Mode().Perm()); err != nil {
			return errors.Join(fmt.Errorf("keep the mode of %s: %w", dst, err), tmp.Close())
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close zip %s: %w", tmp.Name(), err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("move zip into place: %w", err)
	}
	return nil
}

// zipTarget follows a symlinked dst and reports an archive already there, so replacing it keeps the link and the
// mode that writing through os.Create would have kept.
func zipTarget(dst string) (string, fs.FileInfo, error) {
	target := dst
	if resolved, err := filepath.EvalSymlinks(dst); err == nil {
		target = resolved
	}
	info, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return target, nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("create %s: %w", dst, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("create %s: is a directory", dst)
	}
	return target, info, nil
}

// createBeside keeps os.Create's umask-governed mode for a new archive, which os.CreateTemp would force to 0600.
func createBeside(dst string) (*os.File, error) {
	dir, base := filepath.Split(dst)
	for i := 0; i < 100; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%s.%d-%d.tmp", base, os.Getpid(), i))
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", dst, err)
		}
		return f, nil
	}
	return nil, fmt.Errorf("create %s: no free temp name beside it", dst)
}

func writeZip(w io.Writer, root string) error {
	zw := zip.NewWriter(w)
	walkErr := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		return addZipEntry(zw, p, filepath.ToSlash(rel))
	})
	if walkErr != nil {
		return errors.Join(fmt.Errorf("zip %s: %w", root, walkErr), zw.Close())
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	return nil
}

func addZipEntry(zw *zip.Writer, path, name string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

func unzip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", src, err)
	}
	defer r.Close()

	root := filepath.Clean(destDir)
	for _, f := range r.File {
		target := filepath.Join(root, filepath.FromSlash(f.Name))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q escapes destination", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := writeZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(f *zip.File, target string) (err error) {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.Close()) }()

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
