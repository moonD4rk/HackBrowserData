package zipfile

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_ZipsOnlyTheNamedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chrome", "Default", "History"), "history")
	writeFile(t, filepath.Join(root, "notes.txt"), "not named")
	archive := filepath.Join(t.TempDir(), "out.zip")

	require.NoError(t, Write(archive, root, []string{"chrome/Default/History"}))
	assert.Equal(t, map[string]string{"chrome/Default/History": "history"}, entries(t, archive))
	assert.Equal(t, []string{"out.zip"}, filesIn(t, filepath.Dir(archive)))
}

func TestWrite_FailureKeepsAnExistingArchive(t *testing.T) {
	out := t.TempDir()
	archive := filepath.Join(out, "out.zip")
	writeFile(t, archive, "precious")

	require.Error(t, Write(archive, t.TempDir(), []string{"missing"}))
	got, err := os.ReadFile(archive)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(got))
	assert.Equal(t, []string{"out.zip"}, filesIn(t, out), "no temp file may survive a failed zip")
}

// TestWrite_IsOwnerOnly keeps an archive as private as the browser files and records it holds.
func TestWrite_IsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no mode bits to check")
	}
	archive := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, Write(archive, t.TempDir(), nil))
	info, err := os.Stat(archive)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWrite_LeavesAPlantedTempFileAlone(t *testing.T) {
	out := t.TempDir()
	archive := filepath.Join(out, "out.zip")
	writeFile(t, archive+".tmp", "planted")
	require.NoError(t, os.Chmod(archive+".tmp", 0o644))

	require.NoError(t, Write(archive, t.TempDir(), nil))
	planted, err := os.ReadFile(archive + ".tmp")
	require.NoError(t, err)
	assert.Equal(t, "planted", string(planted))
	assert.Equal(t, []string{"out.zip", "out.zip.tmp"}, filesIn(t, out))
	if runtime.GOOS != "windows" {
		info, err := os.Stat(archive)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "a planted 0644 file must not set the archive's mode")
	}
}

func TestWrite_DoesNotFollowAPlantedSymlink(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "out.zip")
	victim := filepath.Join(t.TempDir(), "victim")
	writeFile(t, victim, "precious")
	if err := os.Symlink(victim, archive+".tmp"); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}

	require.NoError(t, Write(archive, t.TempDir(), nil))
	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(got))
	info, err := os.Lstat(archive)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the archive must be a file, not the planted symlink")
}

func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	dirEntries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(dirEntries))
	for _, entry := range dirEntries {
		names = append(names, entry.Name())
	}
	return names
}

func entries(t *testing.T, archive string) map[string]string {
	t.Helper()
	zipReader, err := zip.OpenReader(archive)
	require.NoError(t, err)
	defer zipReader.Close()
	contents := make(map[string]string, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		entryReader, err := zipFile.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(entryReader)
		require.NoError(t, err)
		require.NoError(t, entryReader.Close())
		contents[zipFile.Name] = string(data)
	}
	return contents
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}
