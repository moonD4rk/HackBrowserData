package filemanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSession(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)
	defer s.Cleanup()

	assert.DirExists(t, s.TempDir())
	assert.Contains(t, s.TempDir(), "hbd-")
}

func TestSession_Cleanup(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)

	dir := s.TempDir()
	assert.DirExists(t, dir)

	s.Cleanup()
	assert.NoDirExists(t, dir)
}

func TestSession_Acquire_File(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)
	defer s.Cleanup()

	// Create a source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "Login Data")
	require.NoError(t, os.WriteFile(srcFile, []byte("test data"), 0o644))

	// Acquire it
	dst := filepath.Join(s.TempDir(), "Login Data")
	err = s.Acquire(srcFile, dst, false)
	assert.NoError(t, err)

	// Verify copy
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "test data", string(data))
}

func TestSession_Acquire_WAL(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)
	defer s.Cleanup()

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "Cookies")
	require.NoError(t, os.WriteFile(srcFile, []byte("db"), 0o644))
	require.NoError(t, os.WriteFile(srcFile+"-wal", []byte("wal"), 0o644))
	require.NoError(t, os.WriteFile(srcFile+"-shm", []byte("shm"), 0o644))

	dst := filepath.Join(s.TempDir(), "Cookies")
	err = s.Acquire(srcFile, dst, false)
	assert.NoError(t, err)

	// Main file copied
	assert.FileExists(t, dst)
	// WAL and SHM also copied
	assert.FileExists(t, dst+"-wal")
	assert.FileExists(t, dst+"-shm")
}

func TestSession_Acquire_Dir(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)
	defer s.Cleanup()

	// Create a source directory with files
	srcDir := filepath.Join(t.TempDir(), "leveldb")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "000001.ldb"), []byte("data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "LOCK"), []byte(""), 0o644))

	dst := filepath.Join(s.TempDir(), "leveldb")
	err = s.Acquire(srcDir, dst, true)
	assert.NoError(t, err)

	// Data file copied
	assert.FileExists(t, filepath.Join(dst, "000001.ldb"))
	// LOCK file skipped (CopyDir skips "lock" suffix)
}

func TestSession_Acquire_NotFound(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)
	defer s.Cleanup()

	dst := filepath.Join(s.TempDir(), "nope")
	err = s.Acquire("/nonexistent/file", dst, false)
	assert.Error(t, err)
}
