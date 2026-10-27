package snapshot

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Copy copies a file together with its SQLite -wal and -shm sidecars, or a directory minus every entry named *lock.
func Copy(src, dst string, isDir bool) error {
	if isDir {
		return copyDir(src, dst)
	}
	if err := copyFile(src, dst); err != nil {
		// Chromium on Windows holds Cookies under an exclusive lock that a plain read cannot pass.
		if runtime.GOOS != "windows" {
			return fmt.Errorf("copy: %w", err)
		}
		if lockedErr := copyLocked(src, dst); lockedErr != nil {
			return errors.Join(fmt.Errorf("copy: %w", err), fmt.Errorf("locked copy: %w", lockedErr))
		}
	}

	var sidecarErrs []error
	for _, suffix := range []string{"-wal", "-shm"} {
		if !isFile(src + suffix) {
			continue
		}
		if err := copyFile(src+suffix, dst+suffix); err != nil {
			sidecarErrs = append(sidecarErrs, fmt.Errorf("copy %s: %w", suffix, err))
		}
	}
	return errors.Join(sidecarErrs...)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func copyDir(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	return copyEntry(src, dst, info)
}

func copyEntry(src, dst string, info fs.FileInfo) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		return copyChildren(src, dst)
	case info.Mode().IsRegular():
		return copyRegular(src, dst)
	default:
		return nil
	}
}

func copyChildren(src, dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		child := filepath.Join(src, e.Name())
		// LevelDB's LOCK is held by the running browser and means nothing in a copy; any name ending in lock is skipped.
		if strings.HasSuffix(strings.ToLower(child), "lock") {
			continue
		}
		info, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := copyEntry(child, filepath.Join(dst, e.Name()), info); err != nil {
			return err
		}
	}
	return nil
}

// copyRegular skips a file that vanished after its directory was listed: a live LevelDB compacts while it is copied.
func copyRegular(src, dst string) error {
	in, err := os.Open(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
