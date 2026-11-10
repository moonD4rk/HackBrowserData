package browser

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func writeSnapshotFile(t *testing.T, root, relativePath string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(relativePath), 0o600))
	return path
}

func TestInputPath(t *testing.T) {
	root := t.TempDir()
	legacyCookies := writeSnapshotFile(t, root, "chrome/Default/Cookies")
	localState := writeSnapshotFile(t, root, "chrome/Local State")
	dirSnapshot := snapshot.Open(root).Snapshot("chrome")
	input := NewInput(dirSnapshot, Profile{Name: "Default", RelativePath: "Default"}, nil, nil)

	got, err := input.Path(Source{Path: "Network/Cookies", Fallbacks: []string{"Cookies"}})
	require.NoError(t, err)
	assert.Equal(t, legacyCookies, got, "a candidate that was never captured is skipped")

	got, err = input.Path(Source{Path: "Local State", Shared: true})
	require.NoError(t, err)
	assert.Equal(t, localState, got, "a shared source resolves under the user data dir, not the profile")

	_, err = input.Path(Source{Path: "History"})
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestInputPath_FlatLayoutProfile(t *testing.T) {
	root := t.TempDir()
	history := writeSnapshotFile(t, root, "opera/History")
	input := NewInput(snapshot.Open(root).Snapshot("opera"), Profile{Name: "Opera Stable"}, nil, nil)

	got, err := input.Path(Source{Path: "History"})
	require.NoError(t, err)
	assert.Equal(t, history, got)
}

// TestInputPath_FailedCopyIsReported makes a failed copy an error, not a reason to read a lower candidate.
func TestInputPath_FailedCopyIsReported(t *testing.T) {
	from := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(from, "Default", "Network", "Cookies"), 0o700))
	writeSnapshotFile(t, from, "Default/Cookies")

	root, err := snapshot.Temp()
	require.NoError(t, err)
	defer func() { require.NoError(t, root.Close()) }()
	require.NoError(t, root.Capture(context.Background(), []snapshot.File{
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Network/Cookies"},
		{UserDataDirID: "chrome", UserDataDirPath: from, RelativePath: "Default/Cookies"},
	}))

	input := NewInput(root.Snapshot("chrome"), Profile{Name: "Default", RelativePath: "Default"}, nil, nil)
	_, err = input.Path(Source{Path: "Network/Cookies", Fallbacks: []string{"Cookies"}})
	require.Error(t, err)
	assert.NotErrorIs(t, err, fs.ErrNotExist)
}

func TestInputMasterKeys(t *testing.T) {
	dirSnapshot := snapshot.Open(t.TempDir()).Snapshot("chrome")
	assert.Nil(t, NewInput(dirSnapshot, Profile{}, nil, nil).MasterKeys(context.Background()))

	calls := 0
	want := masterkey.Set{masterkey.V10: []byte("k")}
	input := NewInput(dirSnapshot, Profile{}, func(context.Context) masterkey.Set {
		calls++
		return want
	}, nil)
	assert.Equal(t, want, input.MasterKeys(context.Background()))
	assert.Equal(t, 1, calls)
}

func TestSourceCandidatePaths(t *testing.T) {
	profile := Profile{Name: "Default", RelativePath: "Default"}
	assert.Equal(t, []string{"Default/Network/Cookies", "Default/Cookies"}, Source{Path: "Network/Cookies", Fallbacks: []string{"Cookies"}}.CandidatePaths(profile))
	assert.Equal(t, []string{"Local State"}, Source{Path: "Local State", Shared: true}.CandidatePaths(profile))
	assert.Equal(t, []string{"History"}, Source{Path: "History"}.CandidatePaths(Profile{Name: "default"}),
		"a user data dir that is itself the profile")
}
