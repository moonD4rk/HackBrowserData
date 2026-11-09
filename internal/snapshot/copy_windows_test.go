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

func TestCopyPath_FallsBackToTheLockedRead(t *testing.T) {
	from := filepath.Join(t.TempDir(), "cookies.db")
	testData := []byte("cookie data")
	require.NoError(t, os.WriteFile(from, testData, 0o644))

	handle := openExclusive(t, from)
	defer syscall.CloseHandle(handle)

	into := filepath.Join(t.TempDir(), "cookies.db")
	require.NoError(t, copyPath(from, into, false), "copyPath should succeed via the locked-file fallback")

	copied, err := os.ReadFile(into)
	require.NoError(t, err)
	assert.Equal(t, testData, copied)
}

func TestCopyPath_PlainWhenNotLocked(t *testing.T) {
	from := filepath.Join(t.TempDir(), "unlocked.db")
	testData := []byte("unlocked data")
	require.NoError(t, os.WriteFile(from, testData, 0o644))

	into := filepath.Join(t.TempDir(), "unlocked.db")
	require.NoError(t, copyPath(from, into, false))

	copied, err := os.ReadFile(into)
	require.NoError(t, err)
	assert.Equal(t, testData, copied)
}

// openExclusive uses share mode 0, which locks the file the way Chrome's locking_mode=EXCLUSIVE does.
func openExclusive(t *testing.T, path string) syscall.Handle {
	t.Helper()
	srcPtr, err := syscall.UTF16PtrFromString(path)
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
	return handle
}
