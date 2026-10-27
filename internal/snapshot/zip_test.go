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
	src := t.TempDir()
	for rel, data := range files {
		writeFile(t, filepath.Join(src, filepath.FromSlash(rel)), data)
	}

	archive := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, Open(src).Zip(archive))

	zr, err := zip.OpenReader(archive)
	require.NoError(t, err)
	for _, f := range zr.File {
		assert.NotContains(t, f.Name, `\`, "zip entry names are forward-slash")
	}
	require.NoError(t, zr.Close())

	d, err := Unzip(archive)
	require.NoError(t, err)
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(d.Path(), filepath.FromSlash(rel)))
		require.NoError(t, err, rel)
		assert.True(t, bytes.Equal(want, got), "%s: got %d bytes, want %d", rel, len(got), len(want))
	}
	require.NoError(t, d.Close())
	assert.NoDirExists(t, d.Path())
}

func TestUnzip_RejectsZipSlipAndCleansUp(t *testing.T) {
	tmp := privateTempRoot(t)
	archive := zipWith(t, "../escape.txt")

	_, err := Unzip(archive)
	require.Error(t, err)
	entries, err := os.ReadDir(tmp)
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
	require.Error(t, Open(snapshotSource(t)).Zip(out))
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestZip_ModeMatchesOSCreateAndSurvivesReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	src := snapshotSource(t)
	out := t.TempDir()
	probe, err := os.Create(filepath.Join(out, "probe"))
	require.NoError(t, err)
	require.NoError(t, probe.Close())
	want, err := os.Stat(probe.Name())
	require.NoError(t, err)

	fresh := filepath.Join(out, "fresh.zip")
	require.NoError(t, Open(src).Zip(fresh))
	got, err := os.Stat(fresh)
	require.NoError(t, err)
	assert.Equal(t, want.Mode().Perm(), got.Mode().Perm(), "a new archive gets the mode os.Create gives a file")

	private := filepath.Join(out, "private.zip")
	writeFile(t, private, []byte("old"))
	require.NoError(t, os.Chmod(private, 0o600))
	require.NoError(t, Open(src).Zip(private))
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

	require.NoError(t, Open(snapshotSource(t)).Zip(link))
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&fs.ModeSymlink, "the link stays a link")
	zr, err := zip.OpenReader(target)
	require.NoError(t, err)
	defer zr.Close()
	assert.Len(t, zr.File, 1, "the archive lands in the link's target")
}

// snapshotSource is a snapshot directory holding one captured file.
func snapshotSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "chrome", "Local State"), []byte("{}"))
	return src
}

func zipWith(t *testing.T, names ...string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(archive)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for _, name := range names {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(name))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	return archive
}

func TestZip_ReplacesAnExistingArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "browser-data.zip")
	writeFile(t, archive, []byte("stale"))

	require.NoError(t, Open(snapshotSource(t)).Zip(archive))
	zr, err := zip.OpenReader(archive)
	require.NoError(t, err)
	defer zr.Close()
	require.Len(t, zr.File, 1)
	assert.Equal(t, "chrome/Local State", zr.File[0].Name)
}

// privateTempRoot points os.TempDir at a fresh directory, so a test can see everything Temp leaves behind.
func privateTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, env := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(env, dir)
	}
	return dir
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}
