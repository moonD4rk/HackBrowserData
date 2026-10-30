//go:build windows

package snapshot

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopy_FallsBackToTheLockedRead(t *testing.T) {
	src := filepath.Join(t.TempDir(), "cookies.db")
	testData := []byte("cookie data")
	require.NoError(t, os.WriteFile(src, testData, 0o644))

	handle := openExclusive(t, src)
	defer syscall.CloseHandle(handle)

	dst := filepath.Join(t.TempDir(), "cookies.db")
	require.NoError(t, Copy(src, dst, false), "Copy should succeed via the locked-file fallback")

	copied, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, testData, copied)
}

func TestCopy_PlainWhenNotLocked(t *testing.T) {
	src := filepath.Join(t.TempDir(), "unlocked.db")
	testData := []byte("unlocked data")
	require.NoError(t, os.WriteFile(src, testData, 0o644))

	dst := filepath.Join(t.TempDir(), "unlocked.db")
	require.NoError(t, Copy(src, dst, false))

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
