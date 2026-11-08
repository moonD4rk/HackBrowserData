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

func writeUserDataDir(t *testing.T, home, browserName string) string {
	t.Helper()
	namedBrowser := lookup(t, browserName)
	require.NotEmpty(t, namedBrowser.Platform().UserDataDirs, browserName)
	path := filepath.Join(home, filepath.FromSlash(namedBrowser.Platform().UserDataDirs[0]))
	touch(t, path, "Default", "Preferences")
	touch(t, path, "Default", "History")
	return path
}

func userDataDirIDs(discovery Discovery) []browser.UserDataDirID {
	var ids []browser.UserDataDirID
	for _, dir := range discovery.Dirs {
		ids = append(ids, dir.ID)
	}
	return ids
}

func TestDiscover(t *testing.T) {
	home := t.TempDir()
	writeUserDataDir(t, home, "edge")
	writeUserDataDir(t, home, "chrome")

	discovery := Discover(Options{BrowserName: "all", HomeDir: home})
	assert.Equal(t, []browser.UserDataDirID{"chrome", "edge"}, userDataDirIDs(discovery), "table order, which is output order")
	assert.Empty(t, discovery.Errors)

	discovery = Discover(Options{BrowserName: "edge", HomeDir: home})
	assert.Equal(t, []browser.UserDataDirID{"edge"}, userDataDirIDs(discovery))
}

func TestDiscover_ProfilePathNeedsOneBrowser(t *testing.T) {
	home := t.TempDir()
	writeUserDataDir(t, home, "chrome")
	custom := t.TempDir()
	touch(t, custom, "Default", "Preferences")
	touch(t, custom, "Default", "History")

	discovery := Discover(Options{BrowserName: "chrome", HomeDir: home, ProfilePath: custom})
	require.Len(t, discovery.Dirs, 1)
	assert.Equal(t, custom, discovery.Dirs[0].Path)

	discovery = Discover(Options{BrowserName: "all", HomeDir: home, ProfilePath: custom})
	require.Len(t, discovery.Dirs, 1)
	assert.NotEqual(t, custom, discovery.Dirs[0].Path, "-p is ignored without a specific browser")
}

// TestDiscover_KeepsOnlyProfilesWithData drops a never-used profile, which holds only its Preferences marker.
func TestDiscover_KeepsOnlyProfilesWithData(t *testing.T) {
	home := t.TempDir()
	chrome := writeUserDataDir(t, home, "chrome")
	touch(t, chrome, "Profile 1", "Preferences")
	edge := writeUserDataDir(t, home, "edge")
	require.NoError(t, os.Remove(filepath.Join(edge, "Default", "History")))

	discovery := Discover(Options{BrowserName: "all", HomeDir: home})
	assert.Equal(t, []browser.UserDataDirID{"chrome"}, userDataDirIDs(discovery))
	assert.Equal(t, []browser.Profile{{Name: "Default", RelativePath: "Default"}}, discovery.Dirs[0].Profiles)
}

func TestDiscover_UnreadableDirIsReported(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	path := writeUserDataDir(t, home, "chrome")
	require.NoError(t, os.Chmod(path, 0o000))
	defer func() { require.NoError(t, os.Chmod(path, 0o700)) }()

	discovery := Discover(Options{BrowserName: "all", HomeDir: home})
	assert.Empty(t, discovery.Dirs)
	require.Len(t, discovery.Errors, 1)
	assert.Equal(t, Extraction{UserDataDirID: "chrome"}, discovery.Errors[0].Extraction)
	assert.Equal(t, OutcomeError, discovery.Errors[0].Outcome)
}

// TestWithData_KeepsAProfileItCannotCheck keeps it for the plan to report, since dropping it would hide the failure.
func TestWithData_KeepsAProfileItCannotCheck(t *testing.T) {
	dir := chromeDir(t, "chrome", "Default")
	lock(t, filepath.Join(dir.Path, "Default"))
	assert.Len(t, withData([]browser.UserDataDir{dir}), 1)
}
