package filemanager

import (
	"fmt"
	"os"

	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
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

// Acquire copies a browser file (or directory) from src to dst with the snapshot copier.
func (s *Session) Acquire(src, dst string, isDir bool) error {
	return snapshot.Copy(src, dst, isDir)
}

// Cleanup removes the session's temporary directory and all its contents.
func (s *Session) Cleanup() {
	os.RemoveAll(s.tempDir)
}
