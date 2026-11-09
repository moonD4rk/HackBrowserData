package zipfile

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestExtract_RoundTripsWhatWriteZipped(t *testing.T) {
	files := map[string]string{
		"chrome/Default/History": "history",
		"chrome/Local State":     "{}",
		"empty":                  "",
		"big.bin":                strings.Repeat("A", 3<<20), // more than one chunk of Extract's copy loop
	}
	root := t.TempDir()
	names := make([]string, 0, len(files))
	for name, data := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), data)
		names = append(names, name)
	}
	archive := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, Write(archive, root, names))

	dir := t.TempDir()
	require.NoError(t, Extract(archive, dir))
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		require.NoError(t, err, name)
		assert.Truef(t, bytes.Equal([]byte(want), got), "%s: got %d bytes, want %d", name, len(got), len(want))
	}
}

func TestExtract_RejectsAnEntryOutsideTheDir(t *testing.T) {
	for _, name := range []string{"../escape.txt", "/abs/escape.txt", "a/../../escape.txt"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "snap")
			require.Error(t, Extract(zipWith(t, name), dir))
			assert.NoFileExists(t, filepath.Join(parent, "escape.txt"))
		})
	}
}

func zipWith(t *testing.T, names ...string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "test.zip")
	file, err := os.Create(archive)
	require.NoError(t, err)
	zipWriter := zip.NewWriter(file)
	for _, name := range names {
		writer, err := zipWriter.Create(name)
		require.NoError(t, err)
		_, err = writer.Write([]byte(name))
		require.NoError(t, err)
	}
	require.NoError(t, zipWriter.Close())
	require.NoError(t, file.Close())
	return archive
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
