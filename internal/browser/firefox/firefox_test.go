package firefox

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func testDef(userDataDirs ...string) browser.Def {
	p := browser.Platform{UserDataDirs: userDataDirs}
	return browser.Def{ID: "firefox", Name: "Firefox", Base: browser.Firefox, Darwin: p, Linux: p, Windows: p}
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
		dir := filepath.Join(root, "firefox", filepath.Dir(filepath.FromSlash(rel)))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		installFile(t, dir, src, filepath.Base(rel))
	}
	return snapshot.Open(root).Snapshot("firefox")
}

func TestDiscover_ProfilesAreDirsWithDataFiles(t *testing.T) {
	home := t.TempDir()
	touch(t, home, "Profiles", "abcd1234.default-release", "places.sqlite")
	touch(t, home, "Profiles", "wxyz5678.work", "logins.json")
	touch(t, home, "Profiles", "k3y0nly0.default", "key4.db")
	touch(t, home, "Profiles", "Crash Reports", "InstallTime20240101000000")
	touch(t, home, "Profiles", "profiles.ini")

	dirs, err := Discover(testDef("Profiles"), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.DirID("firefox"), dirs[0].ID)
	assert.Equal(t, filepath.Join(home, "Profiles"), dirs[0].Path)
	assert.Equal(t, []browser.Profile{
		{Name: "abcd1234.default-release", Dir: "abcd1234.default-release"},
		{Name: "wxyz5678.work", Dir: "wxyz5678.work"},
	}, dirs[0].Profiles)
}

func TestDiscover_MissingUserDataDirYieldsNothing(t *testing.T) {
	dirs, err := Discover(testDef("Profiles"), browser.DiscoverOptions{Home: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// A user data dir that exists but cannot be read must not look like a browser that is not installed.
func TestDiscover_UnreadableUserDataDirIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	touch(t, home, "Profiles", "abcd1234.default-release", "places.sqlite")
	profiles := filepath.Join(home, "Profiles")
	require.NoError(t, os.Chmod(profiles, 0o000))
	defer func() { require.NoError(t, os.Chmod(profiles, 0o700)) }()

	dirs, err := Discover(testDef("Profiles"), browser.DiscoverOptions{Home: home})
	require.Error(t, err)
	assert.Empty(t, dirs)
}

// Before, -p searched its parent and exported every profile beside the one it named.
func TestDiscover_ProfilePathSelectsOnlyThatProfile(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "abcd1234.default-release", "places.sqlite")
	touch(t, custom, "wxyz5678.work", "places.sqlite")

	opts := browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: filepath.Join(custom, "abcd1234.default-release")}
	dirs, err := Discover(testDef("Profiles"), opts)
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, custom, dirs[0].Path)
	assert.Equal(t, []browser.Profile{{Name: "abcd1234.default-release", Dir: "abcd1234.default-release"}}, dirs[0].Profiles)
}

func TestDiscover_ProfilePathWithoutDataFindsNothing(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "abcd1234.default-release", "key4.db")

	opts := browser.DiscoverOptions{Home: t.TempDir(), ProfilePath: filepath.Join(custom, "abcd1234.default-release")}
	dirs, err := Discover(testDef("Profiles"), opts)
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestExtractors_CoverFirefoxCategories(t *testing.T) {
	ex := Extractors(browser.UserDataDir{Def: testDef()}, browser.Profile{})
	for _, c := range category.All() {
		e, ok := ex[c]
		if c == category.CreditCard || c == category.SessionStorage {
			assert.Falsef(t, ok, "Firefox has no %s extractor", c)
			continue
		}
		if assert.Truef(t, ok, "no extractor for %s", c) {
			assert.NotNil(t, e.Extract, c)
			assert.NotEmpty(t, e.Sources, c)
		}
	}
}

// Without key4.db the logins are still exported with empty secrets, and the missing key is the first problem; before,
// decrypting a legacy-cipher login without a key panicked and ended the run.
func TestExtractLogins_WithoutKeyStore(t *testing.T) {
	enc := loginPBEBase64(t)
	logins := createTestJSON(t, "logins.json", fmt.Sprintf(`{"logins": [{"hostname": "https://a.test",
		"encryptedUsername": %q, "encryptedPassword": %q, "timeCreated": 1700000000000}]}`, enc, enc))
	ps := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"p/logins.json": logins}),
		browser.Profile{Name: "p", Dir: "p"}, nil)

	var out testCollector
	require.NoError(t, extractLogins(context.Background(), ps, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 1)
	assert.Equal(t, "https://a.test", got[0].URL)
	assert.Empty(t, got[0].Username)
	assert.Empty(t, got[0].Password)
	require.Len(t, out.fails, 3)
	assert.ErrorIs(t, out.fails[0], fs.ErrNotExist)
}

// A profile with no saved logins never needs its key, so an unreadable key4.db is no problem there.
func TestExtractLogins_NoLoginsNeverReadsTheKey(t *testing.T) {
	ps := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{
		"p/logins.json": createTestJSON(t, "logins.json", `{"logins": []}`),
		"p/key4.db":     createTestJSON(t, "key4.db", "not a database"),
	}), browser.Profile{Name: "p", Dir: "p"}, nil)

	var out testCollector
	require.NoError(t, extractLogins(context.Background(), ps, &out))
	assert.Empty(t, out.records)
	assert.Empty(t, out.fails)
}

// One snapshot serves every profile of a user data dir; each profile must read its own files from it.
func TestExtractors_EachProfileReadsItsOwnFiles(t *testing.T) {
	snap := snapshotOf(t, map[string]string{
		"a.default/places.sqlite": createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
			insertMozPlace(1, "https://home.example", "Home", 1, 1700000000000000)),
		"b.work/places.sqlite": createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
			insertMozPlace(1, "https://work.example", "Work", 1, 1700000000000000)),
	})
	dir := browser.UserDataDir{Def: testDef()}
	want := map[string]string{"a.default": "Home", "b.work": "Work"}
	for _, p := range []browser.Profile{{Name: "a.default", Dir: "a.default"}, {Name: "b.work", Dir: "b.work"}} {
		var out testCollector
		require.NoError(t, Extractors(dir, p)[category.History].Extract(context.Background(), browser.NewProfileSnapshot(snap, p, nil), &out))
		got := collected[record.History](t, &out)
		require.Len(t, got, 1, p.Name)
		assert.Equal(t, want[p.Name], got[0].Title, p.Name)
	}
}
