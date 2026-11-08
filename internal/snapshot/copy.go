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

// copyPath copies a file together with its SQLite -wal and -shm sidecars, or a directory minus every entry named *lock.
func copyPath(from, into string, isDir bool) error {
	if isDir {
		return copyDir(from, into)
	}
	if err := copyFile(from, into); err != nil {
		// Chromium on Windows holds Cookies under an exclusive lock that a plain read cannot pass.
		if runtime.GOOS != "windows" {
			return fmt.Errorf("copy: %w", err)
		}
		if lockedErr := copyLocked(from, into); lockedErr != nil {
			return errors.Join(fmt.Errorf("copy: %w", err), fmt.Errorf("locked copy: %w", lockedErr))
		}
	}

	var sidecarErrs []error
	for _, suffix := range []string{"-wal", "-shm"} {
		if !isFile(from + suffix) {
			continue
		}
		if err := copyFile(from+suffix, into+suffix); err != nil {
			sidecarErrs = append(sidecarErrs, fmt.Errorf("copy %s: %w", suffix, err))
		}
	}
	return errors.Join(sidecarErrs...)
}

func copyFile(from, into string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(into, data, 0o600)
}

func copyDir(from, into string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	return copyEntry(from, into, info)
}

func copyEntry(from, into string, info fs.FileInfo) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return os.Symlink(target, into)
	case info.IsDir():
		return copyChildren(from, into)
	case info.Mode().IsRegular():
		return copyRegular(from, into)
	default:
		return nil
	}
}

func copyChildren(from, into string) error {
	if err := os.MkdirAll(into, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(from)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(from, entry.Name())
		// LevelDB's LOCK is held by the running browser and means nothing in a copy; any name ending in lock is skipped.
		if strings.HasSuffix(strings.ToLower(child), "lock") {
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := copyEntry(child, filepath.Join(into, entry.Name()), info); err != nil {
			return err
		}
	}
	return nil
}

// copyRegular skips a file that vanished after its directory was listed: a live LevelDB compacts while it is copied.
func copyRegular(from, into string) error {
	input, err := os.Open(from)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer input.Close()

	out, err := os.OpenFile(into, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, input)
	return errors.Join(err, out.Close())
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
