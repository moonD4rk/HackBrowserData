//go:build windows

package filemanager

import (
	"bytes"
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

	require.NoError(t, os.WriteFile(src, testData, 0o644))

	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

	// Normal copy should fail
	err := copyFile(src, filepath.Join(dir, "normal_copy.db"))
	assert.Error(t, err, "normal copy should fail on exclusively locked file")

	// copyLocked should succeed via DuplicateHandle + FileMapping
	lockedDst := filepath.Join(dir, "locked_copy.db")
	err = copyLocked(src, lockedDst)
	assert.NoError(t, err, "copyLocked should bypass exclusive lock")

	copied, err := os.ReadFile(lockedDst)
	require.NoError(t, err)
	assert.Equal(t, testData, copied, "copied content should match original")
}

func TestCopyLocked_WriteThenRead(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "modified.db")

	// Write initial data
	require.NoError(t, os.WriteFile(src, []byte("initial"), 0o644))

	// Open exclusively and write more data through the handle
	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

	// Seek to end and write additional data
	_, seekErr := syscall.Seek(handle, 0, 2) // SEEK_END
	require.NoError(t, seekErr)
	additional := []byte(" + appended data")
	var written uint32
	writeErr := syscall.WriteFile(handle, additional, &written, nil)
	require.NoError(t, writeErr)
	require.Equal(t, uint32(len(additional)), written)
	flushErr := syscall.FlushFileBuffers(handle)
	require.NoError(t, flushErr)

	// copyLocked should read the full content including appended data
	lockedDst := filepath.Join(dir, "modified_copy.db")
	copyErr := copyLocked(src, lockedDst)
	assert.NoError(t, copyErr)

	copied, err := os.ReadFile(lockedDst)
	require.NoError(t, err)
	assert.Equal(t, "initial + appended data", string(copied),
		"should read complete content including data written after lock")
}

func TestCopyLocked_LargeFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "large.db")

	// Create a file similar in size to a real Cookies database (~64KB)
	data := make([]byte, 65536)
	for i := range data {
		data[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(src, data, 0o644))

	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

	lockedDst := filepath.Join(dir, "large_copy.db")
	err := copyLocked(src, lockedDst)
	assert.NoError(t, err)

	copied, err := os.ReadFile(lockedDst)
	require.NoError(t, err)
	assert.Equal(t, len(data), len(copied), "file sizes should match")
	assert.True(t, bytes.Equal(data, copied), "file content should match byte-for-byte")
}

func TestCopyLocked_FileNotFound(t *testing.T) {
	err := copyLocked("/nonexistent/file.db", filepath.Join(t.TempDir(), "dst.db"))
	assert.Error(t, err)
}

func TestAcquire_FallbackToLocked(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cookies.db")
	testData := []byte("cookie data")

	require.NoError(t, os.WriteFile(src, testData, 0o644))

	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

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

func TestAcquire_NormalCopyWhenNotLocked(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "unlocked.db")
	testData := []byte("unlocked data")

	require.NoError(t, os.WriteFile(src, testData, 0o644))

	// No exclusive lock — normal copy should work without needing copyLocked
	session, err := NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	dst := filepath.Join(session.TempDir(), "unlocked.db")
	err = session.Acquire(src, dst, false)
	assert.NoError(t, err)

	copied, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, testData, copied)
}

// openExclusive opens a file with exclusive lock (dwShareMode=0),
// simulating Chrome's PRAGMA locking_mode=EXCLUSIVE behavior.
func openExclusive(t *testing.T, path string) syscall.Handle {
	t.Helper()
	srcPtr, err := syscall.UTF16PtrFromString(path)
	require.NoError(t, err)

	handle, err := syscall.CreateFile(
		srcPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, // exclusive: no sharing
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	require.NoError(t, err)
	return handle
}
