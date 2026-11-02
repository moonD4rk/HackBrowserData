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
	assert.NoFileExists(t, archive+".tmp")
}

func TestWrite_FailureKeepsAnExistingArchive(t *testing.T) {
	out := t.TempDir()
	archive := filepath.Join(out, "out.zip")
	writeFile(t, archive, "precious")

	require.Error(t, Write(archive, t.TempDir(), []string{"missing"}))
	got, err := os.ReadFile(archive)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(got))
	assert.NoFileExists(t, archive+".tmp", "no temp file may survive a failed zip")
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
