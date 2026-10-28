package chromium

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func testDef(id string, userDataDirs ...string) browser.Def {
	p := browser.Platform{UserDataDirs: userDataDirs, KeychainLabel: "Test Safe Storage"}
	return browser.Def{ID: id, Name: id, Base: browser.Chromium, Darwin: p, Linux: p, Windows: p}
}

func touch(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
}

// snapshotOf lays fixture files out as one user data dir's snapshot, keyed by their path relative to the dir.
func snapshotOf(t *testing.T, files map[string]string) snapshot.Snapshot {
	t.Helper()
	root := t.TempDir()
	for rel, src := range files {
		dir := filepath.Join(root, "chrome", filepath.Dir(filepath.FromSlash(rel)))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		installFile(t, dir, src, filepath.Base(rel))
	}
	return snapshot.Open(root).Snapshot("chrome")
}

func TestDiscover_FindsMarkedProfiles(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"Default", "Profile 1", "System Profile", "Guest Profile", "Snapshot"} {
		touch(t, home, "ud", name, "Preferences")
	}
	touch(t, home, "ud", "QQ", "Preferences_02")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ud", "Crashpad"), 0o700))

	dirs, err := Discover(testDef("chrome", "ud"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.DirID("chrome"), dirs[0].ID)
	assert.Equal(t, home+"/ud", dirs[0].Path, "joined with a slash, as keys.json has always recorded it")
	assert.Equal(t, []browser.Profile{
		{Name: "Default", Dir: "Default"},
		{Name: "Profile 1", Dir: "Profile 1"},
		{Name: "QQ", Dir: "QQ"},
	}, dirs[0].Profiles)
}

// Two Arc packages are two user data dirs; their IDs follow path order, so they are the same on every run.
func TestDiscover_EveryGlobMatchIsItsOwnUserDataDir(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "Packages", "Arc_b", "User Data", "Default", "Preferences")
	touch(t, home, "Packages", "Arc_a", "User Data", "Default", "Preferences")

	dirs, err := Discover(testDef("arc", "Packages/Arc_*/User Data"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 2)
	assert.Equal(t, browser.DirID("arc"), dirs[0].ID)
	assert.Equal(t, filepath.Join(home, "Packages", "Arc_a", "User Data"), dirs[0].Path)
	assert.Equal(t, browser.DirID("arc-2"), dirs[1].ID)
	assert.Equal(t, filepath.Join(home, "Packages", "Arc_b", "User Data"), dirs[1].Path)
	assert.Equal(t, "arc", dirs[1].Def.ID, "the browser stays arc; only the directory is numbered")
}

func TestDiscover_FlatLayoutIsOneProfile(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "Opera Stable", "Login Data")

	dirs, err := Discover(testDef("opera", "Opera Stable"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, []browser.Profile{{Name: "Opera Stable"}}, dirs[0].Profiles)
}

func TestDiscover_CopiedTreeWithoutMarkers(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "ud", "Default", "History")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ud", "Crashpad"), 0o700))

	dirs, err := Discover(testDef("chrome", "ud"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, []browser.Profile{{Name: "Default", Dir: "Default"}}, dirs[0].Profiles)
}

func TestDiscover_MissingUserDataDirYieldsNothing(t *testing.T) {
	dirs, err := Discover(testDef("chrome", "ud"), browser.DiscoverOptions{Home: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// A user data dir that exists but cannot be read must not look like a browser that is not installed.
func TestDiscover_UnreadableUserDataDirIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	touch(t, home, "ud", "Default", "Preferences")
	ud := filepath.Join(home, "ud")
	require.NoError(t, os.Chmod(ud, 0o000))
	defer func() { require.NoError(t, os.Chmod(ud, 0o700)) }()

	dirs, err := Discover(testDef("chrome", "ud"), browser.DiscoverOptions{Home: home})
	require.Error(t, err)
	assert.Empty(t, dirs)
}

func TestDiscover_MalformedGlobIsAnError(t *testing.T) {
	_, err := Discover(testDef("chrome", "ud["), browser.DiscoverOptions{Home: t.TempDir()})
	require.Error(t, err)
}

func TestDiscover_ProfilePathReplacesTheTable(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "Default", "Preferences")

	dirs, err := Discover(testDef("chrome", "ud"), browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: custom})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, custom, dirs[0].Path)
}

// Before, a profile directory became the whole user data dir, so the Local State one level up was never found.
func TestDiscover_ProfilePathAtAProfileSelectsOnlyIt(t *testing.T) {
	userData := t.TempDir()
	touch(t, userData, "Local State")
	touch(t, userData, "Default", "Preferences")
	touch(t, userData, "Profile 1", "Preferences")

	opts := browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: filepath.Join(userData, "Profile 1") + string(filepath.Separator)}
	dirs, err := Discover(testDef("chrome", "ud"), opts)
	require.NoError(t, err)
	want := browser.UserDataDir{
		ID: "chrome", Def: testDef("chrome", "ud"), Path: userData,
		Profiles: []browser.Profile{{Name: "Profile 1", Dir: "Profile 1"}},
	}
	assert.Equal(t, []browser.UserDataDir{want}, dirs)
}

// A flat layout keeps Local State beside its Preferences: it is a user data dir, as before.
func TestDiscover_ProfilePathAtAFlatUserDataDir(t *testing.T) {
	flat := t.TempDir()
	touch(t, flat, "Local State")
	touch(t, flat, "Preferences")
	touch(t, flat, "History")

	dirs, err := Discover(testDef("opera", "ud"), browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: flat})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, flat, dirs[0].Path)
	assert.Equal(t, []browser.Profile{{Name: filepath.Base(flat)}}, dirs[0].Profiles)
}

func TestExtractors_CoverEveryCategory(t *testing.T) {
	ex := Extractors(browser.UserDataDir{Def: testDef("chrome")}, browser.Profile{})
	for _, c := range category.All() {
		e, ok := ex[c]
		if assert.Truef(t, ok, "no extractor for %s", c) {
			assert.NotNil(t, e.Extract, c)
			assert.NotEmpty(t, e.Sources, c)
		}
	}
}

func TestOverrides_KeysAreChromiumBrowsers(t *testing.T) {
	for id := range overrides {
		def, ok := browser.Lookup(id)
		if assert.Truef(t, ok, "override %q names no browser", id) {
			assert.Equalf(t, browser.Chromium, def.Base, "override %q is not a Chromium browser", id)
		}
	}
}

func TestExtractors_OverridesFollowTheBrowserNotTheDirectory(t *testing.T) {
	yandex, ok := browser.Lookup("yandex")
	require.True(t, ok)
	ex := Extractors(browser.UserDataDir{ID: "yandex-2", Def: yandex}, browser.Profile{})
	assert.Equal(t, []string{"Ya Passman Data"}, ex[category.Password].Sources[0].Paths)
	assert.Equal(t, []string{"Ya Credit Cards"}, ex[category.CreditCard].Sources[0].Paths)
}

func TestExtractors_OperaReadsOpsettings(t *testing.T) {
	prefs := createTestJSON(t, "Secure Preferences", `{
		"extensions": {"opsettings": {"ext1": {"location": 1, "manifest": {"name": "Opera Ext", "version": "1.0"}}}}
	}`)
	ps := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"Default/Secure Preferences": prefs}),
		browser.Profile{Name: "Default", Dir: "Default"}, nil)

	for _, id := range []string{"opera", "opera-gx", "vought"} {
		def, ok := browser.Lookup(id)
		require.True(t, ok, id)
		var out testCollector
		require.NoError(t, Extractors(browser.UserDataDir{Def: def}, ps.Profile)[category.Extension].Extract(context.Background(), ps, &out), id)
		got := collected[record.Extension](t, &out)
		require.Len(t, got, 1, id)
		assert.Equal(t, "Opera Ext", got[0].Name, id)
	}

	chrome, ok := browser.Lookup("chrome")
	require.True(t, ok)
	err := Extractors(browser.UserDataDir{Def: chrome}, ps.Profile)[category.Extension].Extract(context.Background(), ps, &testCollector{})
	require.Error(t, err, "Chrome has no opsettings fallback")
}

func TestExtractPasswordStores_LocalBeforeAccount(t *testing.T) {
	local := createTestDB(t, "Login Data", loginsSchema, insertLogin("https://local.example", "", "local-user", "", 13340000000000000))
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	ps := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{
		"Default/Login Data":             local,
		"Default/Login Data For Account": account,
	}), browser.Profile{Name: "Default", Dir: "Default"}, nil)

	var out testCollector
	require.NoError(t, extractPasswordStores(context.Background(), ps, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 2)
	assert.Equal(t, "local-user", got[0].Username)
	assert.Equal(t, record.StoreLocal, got[0].Store)
	assert.Equal(t, "account-user", got[1].Username)
	assert.Equal(t, record.StoreAccount, got[1].Store)
}

func TestExtractPasswordStores_AccountOnly(t *testing.T) {
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	ps := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"Default/Login Data For Account": account}),
		browser.Profile{Name: "Default", Dir: "Default"}, nil)

	var out testCollector
	require.NoError(t, extractPasswordStores(context.Background(), ps, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 1)
	assert.Equal(t, record.StoreAccount, got[0].Store)
}

func TestExtractPasswordStores_OneBrokenDatabaseKeepsTheOther(t *testing.T) {
	broken := createTestJSON(t, "Login Data", "not a database")
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	ps := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{
		"Default/Login Data":             broken,
		"Default/Login Data For Account": account,
	}), browser.Profile{Name: "Default", Dir: "Default"}, nil)

	var out testCollector
	require.Error(t, extractPasswordStores(context.Background(), ps, &out))
	assert.Len(t, collected[record.Login](t, &out), 1)
}

// One snapshot serves every profile of a user data dir; each profile must read its own files from it.
func TestExtractors_EachProfileReadsItsOwnFiles(t *testing.T) {
	snap := snapshotOf(t, map[string]string{
		"Default/History":   createTestDB(t, "History", urlsSchema, insertURL("https://home.example", "Home", 1, 13350000000000000)),
		"Profile 1/History": createTestDB(t, "History", urlsSchema, insertURL("https://work.example", "Work", 1, 13350000000000000)),
	})
	dir := browser.UserDataDir{Def: testDef("chrome")}
	want := map[string]string{"Default": "Home", "Profile 1": "Work"}
	for _, p := range []browser.Profile{{Name: "Default", Dir: "Default"}, {Name: "Profile 1", Dir: "Profile 1"}} {
		var out testCollector
		require.NoError(t, Extractors(dir, p)[category.History].Extract(context.Background(), browser.NewProfileSnapshot(snap, p, nil), &out))
		got := collected[record.History](t, &out)
		require.Len(t, got, 1, p.Name)
		assert.Equal(t, want[p.Name], got[0].Title, p.Name)
	}
}

func TestKeyLocator(t *testing.T) {
	p := browser.Platform{KeychainLabel: "Test Safe Storage", Exe: "chrome.exe", ABE: masterkey.ABEChromeBase}
	dir := browser.UserDataDir{ID: "chrome", Def: browser.Def{ID: "chrome", Base: browser.Chromium, Darwin: p, Linux: p, Windows: p}}

	snap := snapshotOf(t, map[string]string{"Local State": createTestJSON(t, "Local State", "{}")})
	loc, err := KeyLocator(dir, snap)
	require.NoError(t, err)
	assert.NotEmpty(t, loc.LocalState)
	assert.Equal(t, "Test Safe Storage", loc.KeychainLabel)
	assert.Equal(t, "chrome.exe", loc.Exe)
	assert.Equal(t, masterkey.ABEChromeBase, loc.ABE)

	loc, err = KeyLocator(dir, snapshotOf(t, nil))
	require.NoError(t, err, "a user data dir without Local State still has keys on macOS and Linux")
	assert.Empty(t, loc.LocalState)
}

func TestArchiveSources(t *testing.T) {
	assert.Equal(t, []browser.Source{
		localState,
		{Paths: []string{"Preferences"}, Optional: true},
		{Paths: []string{"Preferences_02"}, Optional: true},
	}, ArchiveSources(browser.UserDataDir{}))
}
