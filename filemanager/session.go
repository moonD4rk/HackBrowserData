package filemanager

import (
	"errors"
	"fmt"
	"os"
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
// If the normal copy fails (e.g. file locked by browser on Windows),
// it falls back to a platform-specific locked-file copy method.
func (s *Session) Acquire(src, dst string, isDir bool) error {
	if isDir {
		return copyDir(src, dst, "lock")
	}

	// Try normal copy first
	err := copyFile(src, dst)
	if err != nil {
		// Normal copy failed, try platform-specific locked file copy
		if err2 := copyLocked(src, dst); err2 != nil {
			return errors.Join(
				fmt.Errorf("copy: %w", err),
				fmt.Errorf("locked copy: %w", err2),
			)
		}
	}

	// Copy SQLite WAL/SHM companion files if present
	for _, suffix := range []string{"-wal", "-shm"} {
		walSrc := src + suffix
		if isFileExists(walSrc) {
			_ = copyFile(walSrc, dst+suffix)
		}
	}
	return nil
}

// Cleanup removes the session's temporary directory and all its contents.
func (s *Session) Cleanup() {
	os.RemoveAll(s.tempDir)
}
