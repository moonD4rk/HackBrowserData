package chromium

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

func touch(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
}

func TestDiscover_FindsMarkedProfiles(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"Default", "Profile 1", "System Profile", "Guest Profile", "Snapshot"} {
		touch(t, home, "ud", name, "Preferences")
	}
	touch(t, home, "ud", "QQ", "Preferences_02")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ud", "Crashpad"), 0o700))

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{HomeDir: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.UserDataDirID("chrome"), dirs[0].ID)
	assert.Equal(t, home+"/ud", dirs[0].Path, "joined with a slash, as keys.json has always recorded it")
	assert.Equal(t, []browser.Profile{
		{Name: "Default", RelativePath: "Default"},
		{Name: "Profile 1", RelativePath: "Profile 1"},
		{Name: "QQ", RelativePath: "QQ"},
	}, dirs[0].Profiles)
}

// TestDiscover_EveryGlobMatchIsItsOwnUserDataDir treats two Arc packages as two user data dirs, numbered by path order.
func TestDiscover_EveryGlobMatchIsItsOwnUserDataDir(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "Packages", "Arc_b", "User Data", "Default", "Preferences")
	touch(t, home, "Packages", "Arc_a", "User Data", "Default", "Preferences")

	dirs, err := Discover(testBrowser("arc", "Packages/Arc_*/User Data"), browser.DiscoverOptions{HomeDir: home})
	require.NoError(t, err)
	require.Len(t, dirs, 2)
	assert.Equal(t, browser.UserDataDirID("arc"), dirs[0].ID)
	assert.Equal(t, filepath.Join(home, "Packages", "Arc_a", "User Data"), dirs[0].Path)
	assert.Equal(t, browser.UserDataDirID("arc-2"), dirs[1].ID)
	assert.Equal(t, filepath.Join(home, "Packages", "Arc_b", "User Data"), dirs[1].Path)
	assert.Equal(t, "arc", dirs[1].Browser.Name, "the browser stays arc; only the directory is numbered")
}

func TestDiscover_FlatLayoutIsOneProfile(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "Opera Stable", "Login Data")

	dirs, err := Discover(testBrowser("opera", "Opera Stable"), browser.DiscoverOptions{HomeDir: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, []browser.Profile{{Name: "Opera Stable"}}, dirs[0].Profiles)
}

func TestDiscover_CopiedTreeWithoutMarkers(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "ud", "Default", "History")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ud", "Crashpad"), 0o700))

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{HomeDir: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, []browser.Profile{{Name: "Default", RelativePath: "Default"}}, dirs[0].Profiles)
}

func TestDiscover_MissingUserDataDirYieldsNothing(t *testing.T) {
	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{HomeDir: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// TestDiscover_UnreadableUserDataDirIsAnError keeps an unreadable dir from looking like a browser not installed.
func TestDiscover_UnreadableUserDataDirIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	touch(t, home, "ud", "Default", "Preferences")
	userDataDir := filepath.Join(home, "ud")
	require.NoError(t, os.Chmod(userDataDir, 0o000))
	defer func() { require.NoError(t, os.Chmod(userDataDir, 0o700)) }()

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{HomeDir: home})
	require.Error(t, err)
	assert.Empty(t, dirs)
}

func TestDiscover_MalformedGlobIsAnError(t *testing.T) {
	_, err := Discover(testBrowser("chrome", "ud["), browser.DiscoverOptions{HomeDir: t.TempDir()})
	require.Error(t, err)
}

func TestDiscover_ProfilePathReplacesTheTable(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "Default", "Preferences")

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{HomeDir: t.TempDir(), ProfilePath: custom})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, custom, dirs[0].Path)
}

// TestDiscover_ProfilePathAtAProfileSelectsOnlyIt guards a fixed bug: a profile dir was taken for its user data dir.
func TestDiscover_ProfilePathAtAProfileSelectsOnlyIt(t *testing.T) {
	userData := t.TempDir()
	touch(t, userData, "Local State")
	touch(t, userData, "Default", "Preferences")
	touch(t, userData, "Profile 1", "Preferences")

	options := browser.DiscoverOptions{HomeDir: t.TempDir(), ProfilePath: filepath.Join(userData, "Profile 1") + string(filepath.Separator)}
	dirs, err := Discover(testBrowser("chrome", "ud"), options)
	require.NoError(t, err)
	want := browser.UserDataDir{
		ID: "chrome", Browser: testBrowser("chrome", "ud"), Path: userData,
		Profiles: []browser.Profile{{Name: "Profile 1", RelativePath: "Profile 1"}},
	}
	assert.Equal(t, []browser.UserDataDir{want}, dirs)
}

// TestDiscover_ProfilePathAtAFlatUserDataDir keeps a flat layout, Local State beside Preferences, a user data dir.
func TestDiscover_ProfilePathAtAFlatUserDataDir(t *testing.T) {
	flat := t.TempDir()
	touch(t, flat, "Local State")
	touch(t, flat, "Preferences")
	touch(t, flat, "History")

	dirs, err := Discover(testBrowser("opera", "ud"), browser.DiscoverOptions{HomeDir: t.TempDir(), ProfilePath: flat})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, flat, dirs[0].Path)
	assert.Equal(t, []browser.Profile{{Name: filepath.Base(flat)}}, dirs[0].Profiles)
}

func TestKeyLocator(t *testing.T) {
	platform := browser.Platform{
		SecretLabel: "Test Safe Storage", Executable: "chrome.exe",
		ExecutableFallbacks: []string{`%ProgramFiles%\Google\Chrome\Application\chrome.exe`}, ABE: masterkey.ABEChromeBase,
	}
	dir := browser.UserDataDir{ID: "chrome", Browser: browser.Browser{Name: "chrome", Base: browser.Chromium, Darwin: platform, Linux: platform, Windows: platform}}

	dirSnapshot := snapshotOf(t, map[string]string{"Local State": createTestJSON(t, "Local State", "{}")})
	locator, err := KeyLocator(dir, dirSnapshot)
	require.NoError(t, err)
	localStatePath, err := dirSnapshot.Path("Local State")
	require.NoError(t, err)
	assert.Equal(t, masterkey.Locator{
		LocalState: localStatePath, SecretLabel: "Test Safe Storage", Executable: "chrome.exe",
		ExecutableFallbacks: platform.ExecutableFallbacks, ABE: masterkey.ABEChromeBase,
	}, locator)

	locator, err = KeyLocator(dir, snapshotOf(t, nil))
	require.NoError(t, err, "a user data dir without Local State still has keys on macOS and Linux")
	assert.Empty(t, locator.LocalState)
}
