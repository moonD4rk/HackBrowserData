package browser

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func writeSnapshotFile(t *testing.T, root, relativePath string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(relativePath), 0o600))
	return path
}

func TestProfileSnapshotPath(t *testing.T) {
	root := t.TempDir()
	legacyCookies := writeSnapshotFile(t, root, "chrome/Default/Cookies")
	localState := writeSnapshotFile(t, root, "chrome/Local State")
	dirSnapshot := snapshot.Open(root).Snapshot("chrome")
	captured := NewProfileSnapshot(UserDataDir{ID: "chrome"}, Profile{Name: "Default", RelativePath: "Default"}, dirSnapshot)

	got, err := captured.Path(Source{RelativePath: "Network/Cookies", Fallbacks: []string{"Cookies"}})
	require.NoError(t, err)
	assert.Equal(t, legacyCookies, got, "a candidate that was never captured is skipped")

	got, err = captured.Path(Source{RelativePath: "Local State", FromUserDataDir: true})
	require.NoError(t, err)
	assert.Equal(t, localState, got, "a source from the user data dir resolves there, not under the profile")

	_, err = captured.Path(Source{RelativePath: "History"})
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestProfileSnapshotPath_FlatLayoutProfile(t *testing.T) {
	root := t.TempDir()
	history := writeSnapshotFile(t, root, "opera/History")
	captured := NewProfileSnapshot(UserDataDir{ID: "opera"}, Profile{Name: "Opera Stable"}, snapshot.Open(root).Snapshot("opera"))

	got, err := captured.Path(Source{RelativePath: "History"})
	require.NoError(t, err)
	assert.Equal(t, history, got)
}

// TestProfileSnapshotPath_FailedCopyIsReported makes a failed copy an error, not a reason to read a lower candidate.
func TestProfileSnapshotPath_FailedCopyIsReported(t *testing.T) {
	from := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(from, "Default", "Network", "Cookies"), 0o700))
	writeSnapshotFile(t, from, "Default/Cookies")

	root, err := snapshot.NewRoot()
	require.NoError(t, err)
	defer func() { require.NoError(t, root.Close()) }()
	require.NoError(t, root.Capture(context.Background(), []snapshot.File{
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Network/Cookies"},
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Cookies"},
	}))

	captured := NewProfileSnapshot(UserDataDir{ID: "chrome"}, Profile{Name: "Default", RelativePath: "Default"}, root.Snapshot("chrome"))
	_, err = captured.Path(Source{RelativePath: "Network/Cookies", Fallbacks: []string{"Cookies"}})
	require.Error(t, err)
	assert.NotErrorIs(t, err, fs.ErrNotExist)
}

func TestSourceDirRelativePaths(t *testing.T) {
	profile := Profile{Name: "Default", RelativePath: "Default"}
	assert.Equal(t, []string{"Default/Network/Cookies", "Default/Cookies"}, Source{RelativePath: "Network/Cookies", Fallbacks: []string{"Cookies"}}.DirRelativePaths(profile))
	assert.Equal(t, []string{"Local State"}, Source{RelativePath: "Local State", FromUserDataDir: true}.DirRelativePaths(profile))
	assert.Equal(t, []string{"History"}, Source{RelativePath: "History"}.DirRelativePaths(Profile{Name: "default"}),
		"a user data dir that is itself the profile")
}
