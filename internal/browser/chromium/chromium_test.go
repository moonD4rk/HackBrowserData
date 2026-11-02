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

func testBrowser(name string, userDataDirs ...string) browser.Browser {
	platform := browser.Platform{UserDataDirs: userDataDirs, KeychainLabel: "Test Safe Storage"}
	return browser.Browser{Name: name, DisplayName: name, Base: browser.Chromium, Darwin: platform, Linux: platform, Windows: platform}
}

func touch(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
}

func snapshotOf(t *testing.T, files map[string]string) snapshot.Snapshot {
	t.Helper()
	root := t.TempDir()
	for relativePath, fixturePath := range files {
		dir := filepath.Join(root, "chrome", filepath.Dir(filepath.FromSlash(relativePath)))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		installFile(t, dir, fixturePath, filepath.Base(relativePath))
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

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.UserDataDirID("chrome"), dirs[0].ID)
	assert.Equal(t, home+"/ud", dirs[0].Path, "joined with a slash, as keys.json has always recorded it")
	assert.Equal(t, []browser.Profile{
		{Name: "Default", Dir: "Default"},
		{Name: "Profile 1", Dir: "Profile 1"},
		{Name: "QQ", Dir: "QQ"},
	}, dirs[0].Profiles)
}

// TestDiscover_EveryGlobMatchIsItsOwnUserDataDir treats two Arc packages as two user data dirs, numbered by path order.
func TestDiscover_EveryGlobMatchIsItsOwnUserDataDir(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "Packages", "Arc_b", "User Data", "Default", "Preferences")
	touch(t, home, "Packages", "Arc_a", "User Data", "Default", "Preferences")

	dirs, err := Discover(testBrowser("arc", "Packages/Arc_*/User Data"), browser.DiscoverOptions{Home: home})
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

	dirs, err := Discover(testBrowser("opera", "Opera Stable"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, []browser.Profile{{Name: "Opera Stable"}}, dirs[0].Profiles)
}

func TestDiscover_CopiedTreeWithoutMarkers(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "ud", "Default", "History")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "ud", "Crashpad"), 0o700))

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, []browser.Profile{{Name: "Default", Dir: "Default"}}, dirs[0].Profiles)
}

func TestDiscover_MissingUserDataDirYieldsNothing(t *testing.T) {
	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{Home: t.TempDir()})
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

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{Home: home})
	require.Error(t, err)
	assert.Empty(t, dirs)
}

func TestDiscover_MalformedGlobIsAnError(t *testing.T) {
	_, err := Discover(testBrowser("chrome", "ud["), browser.DiscoverOptions{Home: t.TempDir()})
	require.Error(t, err)
}

func TestDiscover_ProfilePathReplacesTheTable(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "Default", "Preferences")

	dirs, err := Discover(testBrowser("chrome", "ud"), browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: custom})
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

	options := browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: filepath.Join(userData, "Profile 1") + string(filepath.Separator)}
	dirs, err := Discover(testBrowser("chrome", "ud"), options)
	require.NoError(t, err)
	want := browser.UserDataDir{
		ID: "chrome", Browser: testBrowser("chrome", "ud"), Path: userData,
		Profiles: []browser.Profile{{Name: "Profile 1", Dir: "Profile 1"}},
	}
	assert.Equal(t, []browser.UserDataDir{want}, dirs)
}

// TestDiscover_ProfilePathAtAFlatUserDataDir keeps a flat layout, Local State beside Preferences, a user data dir.
func TestDiscover_ProfilePathAtAFlatUserDataDir(t *testing.T) {
	flat := t.TempDir()
	touch(t, flat, "Local State")
	touch(t, flat, "Preferences")
	touch(t, flat, "History")

	dirs, err := Discover(testBrowser("opera", "ud"), browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: flat})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, flat, dirs[0].Path)
	assert.Equal(t, []browser.Profile{{Name: filepath.Base(flat)}}, dirs[0].Profiles)
}

func TestExtractors_CoverEveryCategory(t *testing.T) {
	extractors := Extractors(browser.UserDataDir{Browser: testBrowser("chrome")}, browser.Profile{})
	for _, dataCategory := range category.All() {
		extractor, ok := extractors[dataCategory]
		if assert.Truef(t, ok, "no extractor for %s", dataCategory) {
			assert.NotNil(t, extractor.Extract, dataCategory)
			assert.NotEmpty(t, extractor.FileSpecs, dataCategory)
		}
	}
}

func TestOverrides_KeysAreChromiumBrowsers(t *testing.T) {
	for name := range overrides {
		chromiumBrowser, ok := browser.Lookup(name)
		if assert.Truef(t, ok, "override %q names no browser", name) {
			assert.Equalf(t, browser.Chromium, chromiumBrowser.Base, "override %q is not a Chromium browser", name)
		}
	}
}

// TestVaultKind_RoundTripsEveryFork checks a kind leads back to its fork, or restore would parse it as plain Chromium.
func TestVaultKind_RoundTripsEveryFork(t *testing.T) {
	assert.Len(t, kinds, len(overrides))
	for name := range overrides {
		kind := VaultKind(browser.Browser{Name: name})
		require.NotEqualf(t, kindChromium, kind, "override %q has no keys.json kind", name)
		forkFormatName, err := FormatName(kind, "unknown")
		require.NoError(t, err)
		assert.Equalf(t, categoriesOf(overrides[name]), categoriesOf(overrides[forkFormatName]), "kind %s of %q", kind, name)
	}
	assert.Equal(t, kindChromium, VaultKind(browser.Browser{Name: "chrome"}))
	plainFormatName, err := FormatName(kindChromium, "unknown")
	require.NoError(t, err)
	assert.Equal(t, "unknown", plainFormatName)
	_, err = FormatName("bogus", "unknown")
	assert.Error(t, err)
}

func categoriesOf(extractors browser.Extractors) []category.Category {
	var out []category.Category
	for _, dataCategory := range category.All() {
		if _, ok := extractors[dataCategory]; ok {
			out = append(out, dataCategory)
		}
	}
	return out
}

func TestExtractors_OverridesFollowTheBrowserNotTheDirectory(t *testing.T) {
	yandex, ok := browser.Lookup("yandex")
	require.True(t, ok)
	extractors := Extractors(browser.UserDataDir{ID: "yandex-2", Browser: yandex}, browser.Profile{})
	assert.Equal(t, []string{"Ya Passman Data"}, extractors[category.Password].FileSpecs[0].Paths)
	assert.Equal(t, []string{"Ya Credit Cards"}, extractors[category.CreditCard].FileSpecs[0].Paths)
}

func TestExtractors_OperaReadsOpsettings(t *testing.T) {
	preferencesPath := createTestJSON(t, "Secure Preferences", `{
		"extensions": {"opsettings": {"ext1": {"location": 1, "manifest": {"name": "Opera Ext", "version": "1.0"}}}}
	}`)
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"Default/Secure Preferences": preferencesPath}),
		browser.Profile{Name: "Default", Dir: "Default"}, nil)

	for _, name := range []string{"opera", "opera-gx", "vought"} {
		chromiumBrowser, ok := browser.Lookup(name)
		require.True(t, ok, name)
		var out testCollector
		require.NoError(t, Extractors(browser.UserDataDir{Browser: chromiumBrowser}, profileSnapshot.Profile)[category.Extension].Extract(context.Background(), profileSnapshot, &out), name)
		got := collected[record.Extension](t, &out)
		require.Len(t, got, 1, name)
		assert.Equal(t, "Opera Ext", got[0].Name, name)
	}

	chrome, ok := browser.Lookup("chrome")
	require.True(t, ok)
	err := Extractors(browser.UserDataDir{Browser: chrome}, profileSnapshot.Profile)[category.Extension].Extract(context.Background(), profileSnapshot, &testCollector{})
	require.Error(t, err, "Chrome has no opsettings fallback")
}

func TestExtractPasswordStores_LocalBeforeAccount(t *testing.T) {
	local := createTestDB(t, "Login Data", loginsSchema, insertLogin("https://local.example", "", "local-user", "", 13340000000000000))
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{
		"Default/Login Data":             local,
		"Default/Login Data For Account": account,
	}), browser.Profile{Name: "Default", Dir: "Default"}, nil)

	var out testCollector
	require.NoError(t, extractPasswordStores(context.Background(), profileSnapshot, &out))
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
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"Default/Login Data For Account": account}),
		browser.Profile{Name: "Default", Dir: "Default"}, nil)

	var out testCollector
	require.NoError(t, extractPasswordStores(context.Background(), profileSnapshot, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 1)
	assert.Equal(t, record.StoreAccount, got[0].Store)
}

func TestExtractPasswordStores_OneBrokenDatabaseKeepsTheOther(t *testing.T) {
	broken := createTestJSON(t, "Login Data", "not a database")
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{
		"Default/Login Data":             broken,
		"Default/Login Data For Account": account,
	}), browser.Profile{Name: "Default", Dir: "Default"}, nil)

	var out testCollector
	require.Error(t, extractPasswordStores(context.Background(), profileSnapshot, &out))
	assert.Len(t, collected[record.Login](t, &out), 1)
}

// TestExtractors_EachProfileReadsItsOwnFiles reads one snapshot shared by every profile; each must get its own files.
func TestExtractors_EachProfileReadsItsOwnFiles(t *testing.T) {
	dirSnapshot := snapshotOf(t, map[string]string{
		"Default/History":   createTestDB(t, "History", urlsSchema, insertURL("https://home.example", "Home", 1, 13350000000000000)),
		"Profile 1/History": createTestDB(t, "History", urlsSchema, insertURL("https://work.example", "Work", 1, 13350000000000000)),
	})
	dir := browser.UserDataDir{Browser: testBrowser("chrome")}
	want := map[string]string{"Default": "Home", "Profile 1": "Work"}
	for _, profile := range []browser.Profile{{Name: "Default", Dir: "Default"}, {Name: "Profile 1", Dir: "Profile 1"}} {
		var out testCollector
		require.NoError(t, Extractors(dir, profile)[category.History].Extract(context.Background(), browser.NewProfileSnapshot(dirSnapshot, profile, nil), &out))
		got := collected[record.History](t, &out)
		require.Len(t, got, 1, profile.Name)
		assert.Equal(t, want[profile.Name], got[0].Title, profile.Name)
	}
}

func TestKeyLocator(t *testing.T) {
	platform := browser.Platform{KeychainLabel: "Test Safe Storage", Exe: "chrome.exe", ABE: masterkey.ABEChromeBase}
	dir := browser.UserDataDir{ID: "chrome", Browser: browser.Browser{Name: "chrome", Base: browser.Chromium, Darwin: platform, Linux: platform, Windows: platform}}

	dirSnapshot := snapshotOf(t, map[string]string{"Local State": createTestJSON(t, "Local State", "{}")})
	locator, err := KeyLocator(dir, dirSnapshot)
	require.NoError(t, err)
	assert.NotEmpty(t, locator.LocalState)
	assert.Equal(t, "Test Safe Storage", locator.KeychainLabel)
	assert.Equal(t, "chrome.exe", locator.Exe)
	assert.Equal(t, masterkey.ABEChromeBase, locator.ABE)

	locator, err = KeyLocator(dir, snapshotOf(t, nil))
	require.NoError(t, err, "a user data dir without Local State still has keys on macOS and Linux")
	assert.Empty(t, locator.LocalState)
}

func TestArchiveFileSpecs(t *testing.T) {
	assert.Equal(t, []browser.FileSpec{
		localState,
		{Paths: []string{"Preferences"}, Optional: true},
		{Paths: []string{"Preferences_02"}, Optional: true},
	}, ArchiveFileSpecs(browser.UserDataDir{}))
}
