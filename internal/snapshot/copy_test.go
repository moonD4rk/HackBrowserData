package snapshot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopy_SkipsEveryEntryEndingInLock(t *testing.T) {
	from := t.TempDir()
	for _, name := range []string{"LOCK", "db.lock", "Block", "LOCKED", "000003.log"} {
		writeFile(t, filepath.Join(from, name), []byte(name))
	}
	writeFile(t, filepath.Join(from, "unlock", "inner"), []byte("inner"))

	into := filepath.Join(t.TempDir(), "leveldb")
	require.NoError(t, Copy(from, into, true))

	for _, skipped := range []string{"LOCK", "db.lock", "Block"} {
		assert.NoFileExists(t, filepath.Join(into, skipped))
	}
	assert.NoDirExists(t, filepath.Join(into, "unlock"), "a directory whose name ends in lock is skipped whole")
	assert.FileExists(t, filepath.Join(into, "LOCKED"))
	assert.FileExists(t, filepath.Join(into, "000003.log"))
}

func TestCopy_MissingSourceFails(t *testing.T) {
	dir := t.TempDir()
	require.Error(t, Copy(filepath.Join(dir, "missing"), filepath.Join(dir, "file"), false))
	require.Error(t, Copy(filepath.Join(dir, "missing"), filepath.Join(dir, "dir"), true))
}

func TestCopy_FileWithoutSidecarsCopiesOnlyTheFile(t *testing.T) {
	from := filepath.Join(t.TempDir(), "History")
	writeFile(t, from, []byte("history"))

	into := filepath.Join(t.TempDir(), "History")
	require.NoError(t, Copy(from, into, false))

	got, err := os.ReadFile(into)
	require.NoError(t, err)
	assert.Equal(t, "history", string(got))
	assert.NoFileExists(t, into+"-wal")
	assert.NoFileExists(t, into+"-shm")
}

func TestCopy_SymlinkInADirectoryStaysALink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}
	from := t.TempDir()
	writeFile(t, filepath.Join(from, "target"), []byte("target"))
	require.NoError(t, os.Symlink("target", filepath.Join(from, "link")))

	into := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, Copy(from, into, true))

	got, err := os.Readlink(filepath.Join(into, "link"))
	require.NoError(t, err)
	assert.Equal(t, "target", got)
}

// TestCopyChildren_VanishedDirectoryCopiesAsEmpty copies a directory removed mid-copy as empty, not failing the tree.
func TestCopyChildren_VanishedDirectoryCopiesAsEmpty(t *testing.T) {
	into := filepath.Join(t.TempDir(), "Origins")
	require.NoError(t, copyChildren(filepath.Join(t.TempDir(), "gone"), into))
	entries, err := os.ReadDir(into)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
