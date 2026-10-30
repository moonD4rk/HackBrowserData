package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
)

// installAt creates a profile where this OS's table expects the browser's user data dir under home.
func installAt(t *testing.T, home, id string) string {
	t.Helper()
	def := lookup(t, id)
	require.NotEmpty(t, def.Platform().UserDataDirs, id)
	path := filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0]))
	touch(t, path, "Default", "Preferences")
	touch(t, path, "Default", "History")
	return path
}

func dirIDs(d Discovery) []browser.DirID {
	var ids []browser.DirID
	for _, dir := range d.Dirs {
		ids = append(ids, dir.ID)
	}
	return ids
}

func TestDiscover(t *testing.T) {
	home := t.TempDir()
	installAt(t, home, "edge")
	installAt(t, home, "chrome")

	d := Discover(Request{Browser: "all", Home: home})
	assert.Equal(t, []browser.DirID{"chrome", "edge"}, dirIDs(d), "table order, which is output order")
	assert.Empty(t, d.Failed)

	d = Discover(Request{Browser: "edge", Home: home})
	assert.Equal(t, []browser.DirID{"edge"}, dirIDs(d))
}

func TestDiscover_ProfilePathNeedsOneBrowser(t *testing.T) {
	home := t.TempDir()
	installAt(t, home, "chrome")
	custom := t.TempDir()
	touch(t, custom, "Default", "Preferences")
	touch(t, custom, "Default", "History")

	d := Discover(Request{Browser: "chrome", Home: home, ProfilePath: custom})
	require.Len(t, d.Dirs, 1)
	assert.Equal(t, custom, d.Dirs[0].Path)

	d = Discover(Request{Browser: "all", Home: home, ProfilePath: custom})
	require.Len(t, d.Dirs, 1)
	assert.NotEqual(t, custom, d.Dirs[0].Path, "-p is ignored without a specific browser")
}

// A profile created and never used holds only its Preferences marker; it is not listed, archived, or keyed, as today.
func TestDiscover_KeepsOnlyProfilesWithData(t *testing.T) {
	home := t.TempDir()
	chrome := installAt(t, home, "chrome")
	touch(t, chrome, "Profile 1", "Preferences")
	edge := installAt(t, home, "edge")
	require.NoError(t, os.Remove(filepath.Join(edge, "Default", "History")))

	d := Discover(Request{Browser: "all", Home: home})
	assert.Equal(t, []browser.DirID{"chrome"}, dirIDs(d))
	assert.Equal(t, []browser.Profile{{Name: "Default", Dir: "Default"}}, d.Dirs[0].Profiles)
}

func TestDiscover_UnreadableDirIsReported(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	path := installAt(t, home, "chrome")
	require.NoError(t, os.Chmod(path, 0o000))
	defer func() { require.NoError(t, os.Chmod(path, 0o700)) }()

	d := Discover(Request{Browser: "all", Home: home})
	assert.Empty(t, d.Dirs)
	require.Len(t, d.Failed, 1)
	assert.Equal(t, Extraction{DirID: "chrome"}, d.Failed[0].Extraction)
	assert.Equal(t, OutcomeError, d.Failed[0].Outcome)
}
