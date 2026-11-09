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

func testBrowser(userDataDirs ...string) browser.Browser {
	platform := browser.Platform{UserDataDirs: userDataDirs}
	return browser.Browser{Name: "firefox", DisplayName: "Firefox", Base: browser.Firefox, Darwin: platform, Linux: platform, Windows: platform}
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
		dir := filepath.Join(root, "firefox", filepath.Dir(filepath.FromSlash(relativePath)))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		installFile(t, dir, fixturePath, filepath.Base(relativePath))
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

	dirs, err := Discover(testBrowser("Profiles"), browser.DiscoverOptions{HomeDir: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.UserDataDirID("firefox"), dirs[0].ID)
	assert.Equal(t, filepath.Join(home, "Profiles"), dirs[0].Path)
	assert.Equal(t, []browser.Profile{
		{Name: "abcd1234.default-release", RelativePath: "abcd1234.default-release"},
		{Name: "wxyz5678.work", RelativePath: "wxyz5678.work"},
	}, dirs[0].Profiles)
}

func TestDiscover_MissingUserDataDirYieldsNothing(t *testing.T) {
	dirs, err := Discover(testBrowser("Profiles"), browser.DiscoverOptions{HomeDir: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// TestDiscover_UnreadableUserDataDirIsAnError keeps an unreadable dir from looking like a browser not installed.
func TestDiscover_UnreadableUserDataDirIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	touch(t, home, "Profiles", "abcd1234.default-release", "places.sqlite")
	profiles := filepath.Join(home, "Profiles")
	require.NoError(t, os.Chmod(profiles, 0o000))
	defer func() { require.NoError(t, os.Chmod(profiles, 0o700)) }()

	dirs, err := Discover(testBrowser("Profiles"), browser.DiscoverOptions{HomeDir: home})
	require.Error(t, err)
	assert.Empty(t, dirs)
}

// TestDiscover_ProfilePathSelectsOnlyThatProfile guards a fixed bug: -p exported every profile beside its own.
func TestDiscover_ProfilePathSelectsOnlyThatProfile(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "abcd1234.default-release", "places.sqlite")
	touch(t, custom, "wxyz5678.work", "places.sqlite")

	options := browser.DiscoverOptions{HomeDir: t.TempDir(), ProfilePath: filepath.Join(custom, "abcd1234.default-release")}
	dirs, err := Discover(testBrowser("Profiles"), options)
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, custom, dirs[0].Path)
	assert.Equal(t, []browser.Profile{{Name: "abcd1234.default-release", RelativePath: "abcd1234.default-release"}}, dirs[0].Profiles)
}

func TestDiscover_ProfilePathWithoutDataFindsNothing(t *testing.T) {
	custom := t.TempDir()
	touch(t, custom, "abcd1234.default-release", "key4.db")

	options := browser.DiscoverOptions{HomeDir: t.TempDir(), ProfilePath: filepath.Join(custom, "abcd1234.default-release")}
	dirs, err := Discover(testBrowser("Profiles"), options)
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

func TestExtractors_CoverFirefoxCategories(t *testing.T) {
	extractors := Extractors(browser.UserDataDir{Browser: testBrowser()}, browser.Profile{})
	for _, dataCategory := range category.All() {
		extractor, ok := extractors[dataCategory]
		if dataCategory == category.CreditCard || dataCategory == category.SessionStorage {
			assert.Falsef(t, ok, "Firefox has no %s extractor", dataCategory)
			continue
		}
		if assert.Truef(t, ok, "no extractor for %s", dataCategory) {
			assert.NotNil(t, extractor.Extract, dataCategory)
			assert.NotEmpty(t, extractor.FileSpecs, dataCategory)
		}
	}
}

// TestExtractLogins_WithoutKeyStore exports empty secrets without key4.db; a legacy-cipher login once panicked.
func TestExtractLogins_WithoutKeyStore(t *testing.T) {
	encodedLogin := loginPBEBase64(t)
	logins := createTestJSON(t, "logins.json", fmt.Sprintf(`{"logins": [{"hostname": "https://a.test",
		"encryptedUsername": %q, "encryptedPassword": %q, "timeCreated": 1700000000000}]}`, encodedLogin, encodedLogin))
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"p/logins.json": logins}),
		browser.Profile{Name: "p", RelativePath: "p"}, nil)

	var out testCollector
	require.NoError(t, extractLogins(context.Background(), profileSnapshot, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 1)
	assert.Equal(t, "https://a.test", got[0].URL)
	assert.Empty(t, got[0].Username)
	assert.Empty(t, got[0].Password)
	require.Len(t, out.fails, 3)
	assert.ErrorIs(t, out.fails[0], fs.ErrNotExist)
}

// TestExtractLogins_NoLoginsNeverReadsTheKey leaves key4.db unread without logins, so an unreadable one is harmless.
func TestExtractLogins_NoLoginsNeverReadsTheKey(t *testing.T) {
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{
		"p/logins.json": createTestJSON(t, "logins.json", `{"logins": []}`),
		"p/key4.db":     createTestJSON(t, "key4.db", "not a database"),
	}), browser.Profile{Name: "p", RelativePath: "p"}, nil)

	var out testCollector
	require.NoError(t, extractLogins(context.Background(), profileSnapshot, &out))
	assert.Empty(t, out.records)
	assert.Empty(t, out.fails)
}

// TestExtractors_EachProfileReadsItsOwnFiles reads one snapshot shared by every profile; each must get its own files.
func TestExtractors_EachProfileReadsItsOwnFiles(t *testing.T) {
	dirSnapshot := snapshotOf(t, map[string]string{
		"a.default/places.sqlite": createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
			insertMozPlace(1, "https://home.example", "Home", 1, 1700000000000000)),
		"b.work/places.sqlite": createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
			insertMozPlace(1, "https://work.example", "Work", 1, 1700000000000000)),
	})
	dir := browser.UserDataDir{Browser: testBrowser()}
	want := map[string]string{"a.default": "Home", "b.work": "Work"}
	for _, profile := range []browser.Profile{{Name: "a.default", RelativePath: "a.default"}, {Name: "b.work", RelativePath: "b.work"}} {
		var out testCollector
		require.NoError(t, Extractors(dir, profile)[category.History].Extract(context.Background(), browser.NewProfileSnapshot(dirSnapshot, profile, nil), &out))
		got := collected[record.History](t, &out)
		require.Len(t, got, 1, profile.Name)
		assert.Equal(t, want[profile.Name], got[0].Title, profile.Name)
	}
}
