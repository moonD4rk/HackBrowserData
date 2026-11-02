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
	for relativePath, data := range map[string]string{
		"Local State":                              "{}",
		"Default/Network/Cookies":                  "db",
		"Default/Network/Cookies-wal":              "wal",
		"Default/Network/Cookies-shm":              "shm",
		"Default/Local Storage/leveldb/CURRENT":    "MANIFEST-000001",
		"Default/Local Storage/leveldb/LOCK":       "",
		"Default/Local Storage/leveldb/000003.log": "log",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(relativePath)), []byte(data))
	}
	return root
}

func TestTemp_IsPrivateAndCloseRemovesIt(t *testing.T) {
	snapshotDir, err := Temp()
	require.NoError(t, err)
	info, err := os.Stat(snapshotDir.Path())
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
	}

	require.NoError(t, snapshotDir.Close())
	assert.NoDirExists(t, snapshotDir.Path())
}

func TestOpen_ReadsInPlaceAndCloseLeavesIt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "chrome", "Local State"), []byte("{}"))

	snapshotDir := Open(dir)
	capturedPath, err := snapshotDir.Snapshot("chrome").Path("Local State")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "chrome", "Local State"), capturedPath)

	require.NoError(t, snapshotDir.Close())
	assert.FileExists(t, capturedPath)
}

func TestCapture_FilesWithSidecarsAndDirsWithoutLocks(t *testing.T) {
	from := userDataDir(t)
	snapshotDir, err := Temp()
	require.NoError(t, err)
	defer snapshotDir.Close()

	require.NoError(t, snapshotDir.Capture(context.Background(), []File{
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Local State"},
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Network/Cookies"},
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Local Storage/leveldb", IsDir: true},
	}))

	dirSnapshot := snapshotDir.Snapshot("chrome")
	cookies, err := dirSnapshot.Path("Default/Network/Cookies")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(snapshotDir.Path(), "chrome", "Default", "Network", "Cookies"), cookies)
	for suffix, want := range map[string]string{"-wal": "wal", "-shm": "shm"} {
		got, err := os.ReadFile(cookies + suffix)
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}

	leveldb, err := dirSnapshot.Path("Default/Local Storage/leveldb")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(leveldb, "CURRENT"))
	assert.FileExists(t, filepath.Join(leveldb, "000003.log"))
	assert.NoFileExists(t, filepath.Join(leveldb, "LOCK"))
}

func TestCapture_KeepsGoingPastFailedFiles(t *testing.T) {
	from := userDataDir(t)
	snapshotDir, err := Temp()
	require.NoError(t, err)
	defer snapshotDir.Close()

	require.NoError(t, snapshotDir.Capture(context.Background(), []File{
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/History"}, // vanished before Capture reached it
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Network"}, // a directory read as a file, so the copy fails
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Local State"},
	}))

	dirSnapshot := snapshotDir.Snapshot("chrome")
	_, err = dirSnapshot.Path("Default/History")
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = dirSnapshot.Path("Default/Network")
	require.Error(t, err)
	require.NotErrorIs(t, err, fs.ErrNotExist, "a failed copy reads back as its own error")

	_, err = dirSnapshot.Path("Local State")
	require.NoError(t, err, "one file's failure does not stop the rest")
}

func TestCapture_StopsWhenCancelled(t *testing.T) {
	from := userDataDir(t)
	snapshotDir, err := Temp()
	require.NoError(t, err)
	defer snapshotDir.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = snapshotDir.Capture(ctx, []File{{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Local State"}})
	require.ErrorIs(t, err, context.Canceled)

	_, err = snapshotDir.Snapshot("chrome").Path("Local State")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestCapture_StaysInsideItsOwnSnapshot(t *testing.T) {
	from := userDataDir(t)
	snapshotDir, err := Temp()
	require.NoError(t, err)
	defer snapshotDir.Close()

	require.NoError(t, snapshotDir.Capture(context.Background(), []File{
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "../edge/Local State"},
		{UserDataDirID: "../chrome", UserDataDirPath: from, RelativePath: "Local State"},
		{UserDataDirID: "chrome/sub", UserDataDirPath: from, RelativePath: "Local State"},
		{UserDataDirID: ".", UserDataDirPath: from, RelativePath: "Local State"},
		{UserDataDirID: "", UserDataDirPath: from, RelativePath: "Local State"},
	}))

	_, err = snapshotDir.Snapshot("chrome").Path("../edge/Local State")
	require.Error(t, err)
	entries, err := os.ReadDir(snapshotDir.Path())
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing may be written outside a file's own snapshot")
}

func TestSnapshot_UncapturedFileIsNotExist(t *testing.T) {
	snapshotDir, err := Temp()
	require.NoError(t, err)
	defer snapshotDir.Close()

	_, err = snapshotDir.Snapshot("chrome").Path("Default/Login Data")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestCapture_ALaterCaptureReplacesAnEarlierFailure(t *testing.T) {
	from := t.TempDir()
	snapshotDir, err := Temp()
	require.NoError(t, err)
	defer snapshotDir.Close()

	require.NoError(t, snapshotDir.Capture(context.Background(), []File{{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/History"}}))
	_, err = snapshotDir.Snapshot("chrome").Path("Default/./History")
	require.ErrorIs(t, err, fs.ErrNotExist, "a failure is found under any spelling of the path")

	writeFile(t, filepath.Join(from, "Default", "History"), []byte("history"))
	require.NoError(t, snapshotDir.Capture(context.Background(), []File{{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/./History"}}))
	_, err = snapshotDir.Snapshot("chrome").Path("Default/History")
	require.NoError(t, err)
}
