package snapshot

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userDataDir builds a Chromium-shaped source tree with a WAL database and a LevelDB directory.
func userDataDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, data := range map[string]string{
		"Local State":                              "{}",
		"Default/Network/Cookies":                  "db",
		"Default/Network/Cookies-wal":              "wal",
		"Default/Network/Cookies-shm":              "shm",
		"Default/Local Storage/leveldb/CURRENT":    "MANIFEST-000001",
		"Default/Local Storage/leveldb/LOCK":       "",
		"Default/Local Storage/leveldb/000003.log": "log",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(data))
	}
	return root
}

func TestTemp_IsPrivateAndCloseRemovesIt(t *testing.T) {
	d, err := Temp()
	require.NoError(t, err)
	info, err := os.Stat(d.Path())
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
	}

	require.NoError(t, d.Close())
	assert.NoDirExists(t, d.Path())
}

func TestOpen_ReadsInPlaceAndCloseLeavesIt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "chrome", "Local State"), []byte("{}"))

	d := Open(dir)
	p, err := d.Snapshot("chrome").Path("Local State")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "chrome", "Local State"), p)

	require.NoError(t, d.Close())
	assert.FileExists(t, p)
}

func TestCapture_FilesWithSidecarsAndDirsWithoutLocks(t *testing.T) {
	from := userDataDir(t)
	d, err := Temp()
	require.NoError(t, err)
	defer d.Close()

	require.NoError(t, d.Capture(context.Background(), []File{
		{DirID: "chrome", From: from, Rel: "Local State"},
		{DirID: "chrome", From: from, Rel: "Default/Network/Cookies"},
		{DirID: "chrome", From: from, Rel: "Default/Local Storage/leveldb", IsDir: true},
	}))

	s := d.Snapshot("chrome")
	cookies, err := s.Path("Default/Network/Cookies")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(d.Path(), "chrome", "Default", "Network", "Cookies"), cookies)
	for suffix, want := range map[string]string{"-wal": "wal", "-shm": "shm"} {
		got, err := os.ReadFile(cookies + suffix)
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}

	leveldb, err := s.Path("Default/Local Storage/leveldb")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(leveldb, "CURRENT"))
	assert.FileExists(t, filepath.Join(leveldb, "000003.log"))
	assert.NoFileExists(t, filepath.Join(leveldb, "LOCK"))
}

func TestCapture_KeepsGoingPastFailedFiles(t *testing.T) {
	from := userDataDir(t)
	d, err := Temp()
	require.NoError(t, err)
	defer d.Close()

	require.NoError(t, d.Capture(context.Background(), []File{
		{DirID: "chrome", From: from, Rel: "Default/History"}, // vanished before Capture reached it
		{DirID: "chrome", From: from, Rel: "Default/Network"}, // a directory read as a file, so the copy fails
		{DirID: "chrome", From: from, Rel: "Local State"},
	}))

	s := d.Snapshot("chrome")
	_, err = s.Path("Default/History")
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = s.Path("Default/Network")
	require.Error(t, err)
	require.NotErrorIs(t, err, fs.ErrNotExist, "a failed copy reads back as its own error")

	_, err = s.Path("Local State")
	require.NoError(t, err, "one file's failure does not stop the rest")
}

func TestCapture_StopsWhenCancelled(t *testing.T) {
	from := userDataDir(t)
	d, err := Temp()
	require.NoError(t, err)
	defer d.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = d.Capture(ctx, []File{{DirID: "chrome", From: from, Rel: "Local State"}})
	require.ErrorIs(t, err, context.Canceled)

	_, err = d.Snapshot("chrome").Path("Local State")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestCapture_StaysInsideItsOwnSnapshot(t *testing.T) {
	from := userDataDir(t)
	d, err := Temp()
	require.NoError(t, err)
	defer d.Close()

	require.NoError(t, d.Capture(context.Background(), []File{
		{DirID: "chrome", From: from, Rel: "../edge/Local State"},
		{DirID: "../chrome", From: from, Rel: "Local State"},
		{DirID: "chrome/sub", From: from, Rel: "Local State"},
		{DirID: ".", From: from, Rel: "Local State"},
		{DirID: "", From: from, Rel: "Local State"},
	}))

	_, err = d.Snapshot("chrome").Path("../edge/Local State")
	require.Error(t, err)
	entries, err := os.ReadDir(d.Path())
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing may be written outside a file's own snapshot")
}

func TestSnapshot_UncapturedFileIsNotExist(t *testing.T) {
	d, err := Temp()
	require.NoError(t, err)
	defer d.Close()

	_, err = d.Snapshot("chrome").Path("Default/Login Data")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestCapture_ALaterCaptureReplacesAnEarlierFailure(t *testing.T) {
	from := t.TempDir()
	d, err := Temp()
	require.NoError(t, err)
	defer d.Close()

	require.NoError(t, d.Capture(context.Background(), []File{{DirID: "chrome", From: from, Rel: "Default/History"}}))
	_, err = d.Snapshot("chrome").Path("Default/./History")
	require.ErrorIs(t, err, fs.ErrNotExist, "a failure is found under any spelling of the path")

	writeFile(t, filepath.Join(from, "Default", "History"), []byte("history"))
	require.NoError(t, d.Capture(context.Background(), []File{{DirID: "chrome", From: from, Rel: "Default/./History"}}))
	_, err = d.Snapshot("chrome").Path("Default/History")
	require.NoError(t, err)
}
