//go:build windows

package winapi

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLockedFile_ExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "locked.db")
	testData := []byte("this is locked file content")

	require.NoError(t, os.WriteFile(src, testData, 0o644))

	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

	_, err := os.ReadFile(src)
	require.Error(t, err, "a plain read should fail on an exclusively locked file")

	data, err := ReadLockedFile(src)
	require.NoError(t, err, "ReadLockedFile should bypass the exclusive lock")
	assert.Equal(t, testData, data, "read content should match original")
}

func TestReadLockedFile_WriteThenRead(t *testing.T) {
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

	// ReadLockedFile should read the full content including appended data
	data, err := ReadLockedFile(src)
	require.NoError(t, err)
	assert.Equal(t, "initial + appended data", string(data),
		"should read complete content including data written after lock")
}

func TestReadLockedFile_LargeFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "large.db")

	// Create a file similar in size to a real Cookies database (~64KB)
	want := make([]byte, 65536)
	for i := range want {
		want[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(src, want, 0o644))

	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

	data, err := ReadLockedFile(src)
	require.NoError(t, err)
	assert.Len(t, data, len(want), "file sizes should match")
	assert.True(t, bytes.Equal(want, data), "file content should match byte-for-byte")
}

func TestReadLockedFile_FileNotFound(t *testing.T) {
	_, err := ReadLockedFile("/nonexistent/file.db")
	require.Error(t, err)
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
