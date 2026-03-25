//go:build windows

package filemanager

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyLocked_ExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "locked.db")
	testData := []byte("this is locked file content")

	// Create source file with test data
	require.NoError(t, os.WriteFile(src, testData, 0o644))

	// Open with exclusive lock (dwShareMode=0), simulating Chrome's PRAGMA locking_mode=EXCLUSIVE
	srcPtr, err := syscall.UTF16PtrFromString(src)
	require.NoError(t, err)

	handle, err := syscall.CreateFile(
		srcPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, // exclusive: no sharing allowed
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	require.NoError(t, err)
	defer syscall.CloseHandle(handle)

	// Normal copy should fail because the file is exclusively locked
	normalDst := filepath.Join(dir, "normal_copy.db")
	err = copyFile(src, normalDst)
	assert.Error(t, err, "normal copy should fail on exclusively locked file")

	// copyLocked should succeed via DuplicateHandle + FileMapping
	lockedDst := filepath.Join(dir, "locked_copy.db")
	err = copyLocked(src, lockedDst)
	assert.NoError(t, err, "copyLocked should bypass exclusive lock")

	// Verify content matches
	copied, err := os.ReadFile(lockedDst)
	require.NoError(t, err)
	assert.Equal(t, testData, copied, "copied content should match original")
}

func TestCopyLocked_FileNotFound(t *testing.T) {
	err := copyLocked("/nonexistent/file.db", filepath.Join(t.TempDir(), "dst.db"))
	assert.Error(t, err, "copyLocked should fail for nonexistent file")
}

func TestAcquire_FallbackToLocked(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cookies.db")
	testData := []byte("cookie data")

	require.NoError(t, os.WriteFile(src, testData, 0o644))

	// Lock the file exclusively
	srcPtr, err := syscall.UTF16PtrFromString(src)
	require.NoError(t, err)

	handle, err := syscall.CreateFile(
		srcPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	require.NoError(t, err)
	defer syscall.CloseHandle(handle)

	// Session.Acquire should automatically fallback to copyLocked
	session, err := NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	dst := filepath.Join(session.TempDir(), "cookies.db")
	err = session.Acquire(src, dst, false)
	assert.NoError(t, err, "Acquire should succeed via locked fallback")

	copied, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, testData, copied)
}
