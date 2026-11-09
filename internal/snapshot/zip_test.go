package snapshot

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZipUnzip_RoundTrip(t *testing.T) {
	files := map[string][]byte{
		"small.txt":                      []byte("hello"),
		"chrome/Default/Network/Cookies": []byte("cookie-bytes"),
	}
	snapshotRoot := t.TempDir()
	for relativePath, data := range files {
		writeFile(t, filepath.Join(snapshotRoot, filepath.FromSlash(relativePath)), data)
	}

	archive := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, Open(snapshotRoot).Zip(archive))

	zipReader, err := zip.OpenReader(archive)
	require.NoError(t, err)
	for _, zipFile := range zipReader.File {
		assert.NotContains(t, zipFile.Name, `\`, "zip entry names are forward-slash")
	}
	require.NoError(t, zipReader.Close())

	snapshotDir, err := Unzip(archive)
	require.NoError(t, err)
	for relativePath, want := range files {
		got, err := os.ReadFile(filepath.Join(snapshotDir.Path(), filepath.FromSlash(relativePath)))
		require.NoError(t, err, relativePath)
		assert.True(t, bytes.Equal(want, got), "%s: got %d bytes, want %d", relativePath, len(got), len(want))
	}
	require.NoError(t, snapshotDir.Close())
	assert.NoDirExists(t, snapshotDir.Path())
}

func TestZip_MissingSnapshotDirFails(t *testing.T) {
	err := Open(filepath.Join(t.TempDir(), "missing")).Zip(filepath.Join(t.TempDir(), "browser-data.zip"))
	require.Error(t, err)
}

func snapshotFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chrome", "Local State"), []byte("{}"))
	return root
}

func TestZip_ReplacesAnExistingArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "browser-data.zip")
	writeFile(t, archive, []byte("stale"))

	require.NoError(t, Open(snapshotFixture(t)).Zip(archive))
	zipReader, err := zip.OpenReader(archive)
	require.NoError(t, err)
	defer zipReader.Close()
	require.Len(t, zipReader.File, 1)
	assert.Equal(t, "chrome/Local State", zipReader.File[0].Name)
}
