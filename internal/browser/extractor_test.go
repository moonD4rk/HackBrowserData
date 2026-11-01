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

func writeSnapshotFile(t *testing.T, root, rel string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(rel), 0o600))
	return path
}

func TestProfileSnapshotPath(t *testing.T) {
	root := t.TempDir()
	legacyCookies := writeSnapshotFile(t, root, "chrome/Default/Cookies")
	localState := writeSnapshotFile(t, root, "chrome/Local State")
	snap := snapshot.Open(root).Snapshot("chrome")
	ps := NewProfileSnapshot(snap, Profile{Name: "Default", Dir: "Default"}, nil)

	got, err := ps.Path(Source{Paths: []string{"Network/Cookies", "Cookies"}})
	require.NoError(t, err)
	assert.Equal(t, legacyCookies, got, "a candidate that was never captured is skipped")

	got, err = ps.Path(Source{Paths: []string{"Local State"}, Shared: true})
	require.NoError(t, err)
	assert.Equal(t, localState, got, "a shared source resolves under the user data dir, not the profile")

	_, err = ps.Path(Source{Paths: []string{"History"}})
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestProfileSnapshotPath_FlatLayoutProfile(t *testing.T) {
	root := t.TempDir()
	history := writeSnapshotFile(t, root, "opera/History")
	ps := NewProfileSnapshot(snapshot.Open(root).Snapshot("opera"), Profile{Name: "Opera Stable"}, nil)

	got, err := ps.Path(Source{Paths: []string{"History"}})
	require.NoError(t, err)
	assert.Equal(t, history, got)
}

// A copy that failed is that extraction's error, not a reason to read a lower-priority candidate instead.
func TestProfileSnapshotPath_FailedCopyIsReported(t *testing.T) {
	from := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(from, "Default", "Network", "Cookies"), 0o700))
	writeSnapshotFile(t, from, "Default/Cookies")

	d, err := snapshot.Temp()
	require.NoError(t, err)
	defer func() { require.NoError(t, d.Close()) }()
	require.NoError(t, d.Capture(context.Background(), []snapshot.File{
		{DirID: "chrome", From: from, Rel: "Default/Network/Cookies"},
		{DirID: "chrome", From: from, Rel: "Default/Cookies"},
	}))

	ps := NewProfileSnapshot(d.Snapshot("chrome"), Profile{Name: "Default", Dir: "Default"}, nil)
	_, err = ps.Path(Source{Paths: []string{"Network/Cookies", "Cookies"}})
	require.Error(t, err)
	assert.NotErrorIs(t, err, fs.ErrNotExist)
}

func TestProfileSnapshotMasterKeys(t *testing.T) {
	snap := snapshot.Open(t.TempDir()).Snapshot("chrome")
	assert.Nil(t, NewProfileSnapshot(snap, Profile{}, nil).MasterKeys(context.Background()))

	calls := 0
	want := masterkey.Set{masterkey.V10: []byte("k")}
	ps := NewProfileSnapshot(snap, Profile{}, func(context.Context) masterkey.Set {
		calls++
		return want
	})
	assert.Equal(t, want, ps.MasterKeys(context.Background()))
	assert.Equal(t, 1, calls)
}

func TestSource_Candidates(t *testing.T) {
	p := Profile{Name: "Default", Dir: "Default"}
	assert.Equal(t, []string{"Default/Network/Cookies", "Default/Cookies"}, Source{Paths: []string{"Network/Cookies", "Cookies"}}.Candidates(p))
	assert.Equal(t, []string{"Local State"}, Source{Paths: []string{"Local State"}, Shared: true}.Candidates(p))
	assert.Equal(t, []string{"History"}, Source{Paths: []string{"History"}}.Candidates(Profile{Name: "default"}),
		"a user data dir that is itself the profile")
}
