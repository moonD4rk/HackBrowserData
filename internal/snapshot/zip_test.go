package snapshot

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZipUnzip_RoundTrip(t *testing.T) {
	files := map[string][]byte{
		"empty.txt":                      {},
		"small.txt":                      []byte("hello"),
		"chrome/Default/Network/Cookies": []byte("cookie-bytes"),
		"sub/big.bin":                    bytes.Repeat([]byte("A"), 3<<20), // exercises the chunked copy loop
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

func TestUnzip_RejectsZipSlipAndCleansUp(t *testing.T) {
	tempRoot := privateTempRoot(t)
	archive := zipWith(t, "../escape.txt")

	_, err := Unzip(archive)
	require.Error(t, err)
	entries, err := os.ReadDir(tempRoot)
	require.NoError(t, err)
	assert.Empty(t, entries, "a rejected archive leaves neither the escaped file nor the snapshot directory behind")
}

func TestUnzip_RejectsASiblingSharingTheRootPrefix(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "snap")

	require.Error(t, unzip(zipWith(t, "../snapx/evil.txt"), root))
	assert.NoFileExists(t, filepath.Join(parent, "snapx", "evil.txt"))
}

func TestZip_FailureLeavesNoFile(t *testing.T) {
	out := t.TempDir()
	err := Open(filepath.Join(t.TempDir(), "missing")).Zip(filepath.Join(out, "browser-data.zip"))
	require.Error(t, err)
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Empty(t, entries, "neither the archive nor its temp file may survive a failed zip")
}

func TestZip_FailureKeepsAnExistingArchive(t *testing.T) {
	out := t.TempDir()
	archive := filepath.Join(out, "browser-data.zip")
	writeFile(t, archive, []byte("precious"))

	require.Error(t, Open(filepath.Join(t.TempDir(), "missing")).Zip(archive))
	got, err := os.ReadFile(archive)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(got))
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp file may survive a failed zip")
}

func TestZip_IntoADirectoryFailsBeforeWriting(t *testing.T) {
	out := t.TempDir()
	require.Error(t, Open(snapshotFixture(t)).Zip(out))
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestZip_ModeMatchesOSCreateAndSurvivesReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	snapshotRoot := snapshotFixture(t)
	out := t.TempDir()
	probe, err := os.Create(filepath.Join(out, "probe"))
	require.NoError(t, err)
	require.NoError(t, probe.Close())
	want, err := os.Stat(probe.Name())
	require.NoError(t, err)

	fresh := filepath.Join(out, "fresh.zip")
	require.NoError(t, Open(snapshotRoot).Zip(fresh))
	got, err := os.Stat(fresh)
	require.NoError(t, err)
	assert.Equal(t, want.Mode().Perm(), got.Mode().Perm(), "a new archive gets the mode os.Create gives a file")

	private := filepath.Join(out, "private.zip")
	writeFile(t, private, []byte("old"))
	require.NoError(t, os.Chmod(private, 0o600))
	require.NoError(t, Open(snapshotRoot).Zip(private))
	got, err = os.Stat(private)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), got.Mode().Perm(), "a replaced archive keeps its mode")
}

func TestZip_WritesThroughASymlinkedArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}
	out := t.TempDir()
	target := filepath.Join(out, "real.zip")
	writeFile(t, target, []byte("old"))
	link := filepath.Join(out, "link.zip")
	require.NoError(t, os.Symlink("real.zip", link))

	require.NoError(t, Open(snapshotFixture(t)).Zip(link))
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&fs.ModeSymlink, "the link stays a link")
	zipReader, err := zip.OpenReader(target)
	require.NoError(t, err)
	defer zipReader.Close()
	assert.Len(t, zipReader.File, 1, "the archive lands in the link's target")
}

func snapshotFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chrome", "Local State"), []byte("{}"))
	return root
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

// privateTempRoot points os.TempDir at a fresh directory, so a test can see everything Temp leaves behind.
func privateTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, variable := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(variable, dir)
	}
	return dir
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}
