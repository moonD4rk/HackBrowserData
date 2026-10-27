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
	src := t.TempDir()
	for _, name := range []string{"LOCK", "db.lock", "Block", "LOCKED", "000003.log"} {
		writeFile(t, filepath.Join(src, name), []byte(name))
	}
	writeFile(t, filepath.Join(src, "unlock", "inner"), []byte("inner"))

	dst := filepath.Join(t.TempDir(), "leveldb")
	require.NoError(t, Copy(src, dst, true))

	for _, skipped := range []string{"LOCK", "db.lock", "Block"} {
		assert.NoFileExists(t, filepath.Join(dst, skipped))
	}
	assert.NoDirExists(t, filepath.Join(dst, "unlock"), "a directory whose name ends in lock is skipped whole")
	assert.FileExists(t, filepath.Join(dst, "LOCKED"))
	assert.FileExists(t, filepath.Join(dst, "000003.log"))
}

func TestCopy_MissingSourceFails(t *testing.T) {
	dir := t.TempDir()
	require.Error(t, Copy(filepath.Join(dir, "missing"), filepath.Join(dir, "file"), false))
	require.Error(t, Copy(filepath.Join(dir, "missing"), filepath.Join(dir, "dir"), true))
}

func TestCopy_FileWithoutSidecarsCopiesOnlyTheFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "History")
	writeFile(t, src, []byte("history"))

	dst := filepath.Join(t.TempDir(), "History")
	require.NoError(t, Copy(src, dst, false))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "history", string(got))
	assert.NoFileExists(t, dst+"-wal")
	assert.NoFileExists(t, dst+"-shm")
}

func TestCopy_SymlinkInADirectoryStaysALink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "target"), []byte("target"))
	require.NoError(t, os.Symlink("target", filepath.Join(src, "link")))

	dst := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, Copy(src, dst, true))

	got, err := os.Readlink(filepath.Join(dst, "link"))
	require.NoError(t, err)
	assert.Equal(t, "target", got)
}

// A directory removed between its parent's listing and its own copy is copied as empty instead of failing the tree.
func TestCopyChildren_VanishedDirectoryCopiesAsEmpty(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "Origins")
	require.NoError(t, copyChildren(filepath.Join(t.TempDir(), "gone"), dst))
	entries, err := os.ReadDir(dst)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
