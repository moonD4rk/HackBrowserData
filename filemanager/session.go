package filemanager

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

// Session manages temporary files for a single browser extraction run.
// It creates an isolated temp directory and provides methods to copy
// browser files into it. Call Cleanup() when done to remove all temp files.
type Session struct {
	tempDir string
}

// NewSession creates a session with a unique temporary directory.
func NewSession() (*Session, error) {
	dir, err := os.MkdirTemp("", "hbd-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	return &Session{tempDir: dir}, nil
}

// TempDir returns the session's temporary directory path.
func (s *Session) TempDir() string {
	return s.tempDir
}

// Acquire copies a browser file (or directory) from src to dst.
// For regular files, it also copies SQLite WAL and SHM companion files
// if they exist. For directories (e.g. LevelDB), it copies the entire
// directory while skipping lock files.
//
// On Windows, if the normal copy fails (e.g. file locked by Chrome),
// it falls back to DuplicateHandle + FileMapping to bypass exclusive locks.
func (s *Session) Acquire(src, dst string, isDir bool) error {
	if isDir {
		return copyDir(src, dst, "lock")
	}

	// Try normal copy first
	err := copyFile(src, dst)
	if err != nil {
		// Only attempt locked-file fallback on Windows where Chrome holds exclusive locks.
		// On other platforms, return the original error directly.
		if runtime.GOOS != "windows" {
			return fmt.Errorf("copy: %w", err)
		}
		if err2 := copyLocked(src, dst); err2 != nil {
			return errors.Join(
				fmt.Errorf("copy: %w", err),
				fmt.Errorf("locked copy: %w", err2),
			)
		}
	}

	// Copy SQLite WAL/SHM companion files if present
	var walErrs []error
	for _, suffix := range []string{"-wal", "-shm"} {
		walSrc := src + suffix
		if isFileExists(walSrc) {
			if err := copyFile(walSrc, dst+suffix); err != nil {
				walErrs = append(walErrs, fmt.Errorf("copy %s: %w", suffix, err))
			}
		}
	}
	return errors.Join(walErrs...)
}

// Cleanup removes the session's temporary directory and all its contents.
func (s *Session) Cleanup() {
	os.RemoveAll(s.tempDir)
}
