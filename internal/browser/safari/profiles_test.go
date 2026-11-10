package safari

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
)

func testBrowser() browser.Browser {
	platform := browser.Platform{UserDataDirs: []string{"Library/Safari"}}
	return browser.Browser{Name: "safari", DisplayName: "Safari", Base: browser.Safari, Darwin: platform, Linux: platform, Windows: platform}
}

func libraryAt(t *testing.T, home string) (container string) {
	t.Helper()
	mkFile(t, home, "Library", "Safari", "History.db")
	return filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library")
}

func discoverProfiles(t *testing.T, home string) []browser.Profile {
	t.Helper()
	dirs, err := Discover(context.Background(), testBrowser(), browser.DiscoverOptions{HomeDir: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.UserDataDirID("safari"), dirs[0].ID)
	assert.Equal(t, filepath.Join(home, "Library"), dirs[0].Path)
	return dirs[0].Profiles
}

func named(name, uuid string) browser.Profile {
	return browser.Profile{Name: name, RelativePath: namedProfilesDir + "/" + uuid}
}

var defaultProfile = browser.Profile{Name: "default", RelativePath: "Safari"}

func TestDiscover_DefaultOnly(t *testing.T) {
	home := t.TempDir()
	libraryAt(t, home)
	assert.Equal(t, []browser.Profile{defaultProfile}, discoverProfiles(t, home))
}

func TestDiscover_MissingSafariYieldsNothing(t *testing.T) {
	dirs, err := Discover(context.Background(), testBrowser(), browser.DiscoverOptions{HomeDir: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// TestDiscover_UnreadableSafariDirIsAnError keeps an unreadable dir from looking like Safari not installed.
func TestDiscover_UnreadableSafariDirIsAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny reads here")
	}
	home := t.TempDir()
	libraryAt(t, home)
	library := filepath.Join(home, "Library")
	require.NoError(t, os.Chmod(library, 0o000))
	defer func() { require.NoError(t, os.Chmod(library, 0o700)) }()

	dirs, err := Discover(context.Background(), testBrowser(), browser.DiscoverOptions{HomeDir: home})
	require.Error(t, err)
	assert.Empty(t, dirs)
}

func TestDiscover_ProfilePathIsTheSafariDir(t *testing.T) {
	const uuid = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	copied := t.TempDir()
	mkFile(t, copied, "Library", "SafariCopy", "History.db")
	container := filepath.Join(copied, "Library", "Containers", "com.apple.Safari", "Data", "Library")
	mkFile(t, container, "Safari", "Profiles", uuid, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{{uuid: uuid, title: "work"}})

	home := t.TempDir()
	libraryAt(t, home)
	options := browser.DiscoverOptions{HomeDir: home, ProfilePath: filepath.Join(copied, "Library", "SafariCopy")}
	dirs, err := Discover(context.Background(), testBrowser(), options)
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, filepath.Join(copied, "Library"), dirs[0].Path)
	assert.Equal(t, []browser.Profile{{Name: "default", RelativePath: "SafariCopy"}, named("work", uuid)}, dirs[0].Profiles)
}

func TestDiscover_WithNamedProfile(t *testing.T) {
	const uuid = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	home := t.TempDir()
	container := libraryAt(t, home)
	mkFile(t, container, "Safari", "Profiles", uuid, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{
		{uuid: "DefaultProfile", title: ""},
		{uuid: uuid, title: "work"},
	})
	assert.Equal(t, []browser.Profile{defaultProfile, named("work", uuid)}, discoverProfiles(t, home))
}

// TestDiscover_ReadsACopyOfSafariTabs puts '#' in the live path, which cuts a SQLite URI: only the copy yields titles.
func TestDiscover_ReadsACopyOfSafariTabs(t *testing.T) {
	const uuid = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	home := filepath.Join(t.TempDir(), "user#1")
	container := libraryAt(t, home)
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{{uuid: uuid, title: "work"}})
	assert.Equal(t, []browser.Profile{defaultProfile, named("work", uuid)}, discoverProfiles(t, home))
}

func TestDiscover_EmptyTitleFallbackToUUID(t *testing.T) {
	const uuid = "ABCDEF01-2345-6789-ABCD-EF0123456789"
	home := t.TempDir()
	container := libraryAt(t, home)
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{{uuid: uuid, title: ""}})
	assert.Equal(t, []browser.Profile{defaultProfile, named("profile-abcdef01", uuid)}, discoverProfiles(t, home))
}

// TestDiscover_OrphanUUIDWithoutDBEntry hides a profile dir SafariTabs.db omits, as a readable DB is authoritative.
func TestDiscover_OrphanUUIDWithoutDBEntry(t *testing.T) {
	const dbUUID = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	const orphanUUID = "11111111-2222-3333-4444-555555555555"
	home := t.TempDir()
	container := libraryAt(t, home)
	mkFile(t, container, "Safari", "Profiles", dbUUID, "History.db")
	mkFile(t, container, "Safari", "Profiles", orphanUUID, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{{uuid: dbUUID, title: "declared"}})
	assert.Equal(t, []browser.Profile{defaultProfile, named("declared", dbUUID)}, discoverProfiles(t, home))
}

func TestDiscover_EmptyDBIsAuthoritative(t *testing.T) {
	home := t.TempDir()
	container := libraryAt(t, home)
	mkFile(t, container, "Safari", "Profiles", "99999999-AAAA-BBBB-CCCC-DDDDDDDDDDDD", "History.db")
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), nil)
	assert.Equal(t, []browser.Profile{defaultProfile}, discoverProfiles(t, home))
}

// TestDiscover_MissingDBFallsBackToReadDir lists the Profiles dir, keeping dirs without data: presence is planning's call.
func TestDiscover_MissingDBFallsBackToReadDir(t *testing.T) {
	const uuid = "11111111-2222-3333-4444-555555555555"
	const empty = "4E2D8DD0-A7D2-4684-939A-898B7675C700"
	home := t.TempDir()
	container := libraryAt(t, home)
	mkFile(t, container, "Safari", "Profiles", uuid, "History.db")
	require.NoError(t, os.MkdirAll(filepath.Join(container, "Safari", "Profiles", empty), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(container, "Safari", "Profiles", "Bogus"), 0o755))
	assert.Equal(t, []browser.Profile{defaultProfile, named("profile-11111111", uuid), named("profile-4e2d8dd0", empty)},
		discoverProfiles(t, home))
}

func TestDiscover_DuplicateTitlesDisambiguate(t *testing.T) {
	const uuidA = "AAAAAAAA-0000-0000-0000-000000000001"
	const uuidB = "BBBBBBBB-0000-0000-0000-000000000002"
	home := t.TempDir()
	container := libraryAt(t, home)
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{
		{uuid: uuidA, title: "team"},
		{uuid: uuidB, title: "team"},
	})
	assert.Equal(t, []browser.Profile{defaultProfile, named("team", uuidA), named("team-2", uuidB)}, discoverProfiles(t, home))
}

func TestDiscover_DefaultProfileSentinelIgnored(t *testing.T) {
	home := t.TempDir()
	container := libraryAt(t, home)
	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{{uuid: defaultProfileSentinel, title: ""}})
	assert.Equal(t, []browser.Profile{defaultProfile}, discoverProfiles(t, home))
}

func TestResolveProfileName(t *testing.T) {
	tests := []struct {
		title string
		uuid  string
		want  string
	}{
		{"work", "5604E6F5-02ED-4E40-8249-63DE7BC986C8", "work"},
		{"  spaced  ", "5604E6F5-02ED-4E40-8249-63DE7BC986C8", "spaced"},
		{"", "5604E6F5-02ED-4E40-8249-63DE7BC986C8", "profile-5604e6f5"},
		{"with/slash", "AAAAAAAA-0000-0000-0000-000000000000", "with_slash"},
		{"中文", "AAAAAAAA-0000-0000-0000-000000000000", "中文"},
	}
	for _, testCase := range tests {
		t.Run(testCase.title, func(t *testing.T) {
			assert.Equal(t, testCase.want, resolveProfileName(testCase.title, testCase.uuid))
		})
	}
}

type tabRow struct {
	uuid  string
	title string
}

// writeSafariTabsDB gives every row subtype=2, the profile record the production query selects.
func writeSafariTabsDB(t *testing.T, path string, rows []tabRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer database.Close()

	_, err = database.Exec(`CREATE TABLE bookmarks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		external_uuid TEXT,
		title TEXT,
		subtype INTEGER DEFAULT 0
	)`)
	require.NoError(t, err)

	for _, row := range rows {
		_, err = database.Exec(`INSERT INTO bookmarks (external_uuid, title, subtype) VALUES (?, ?, 2)`, row.uuid, row.title)
		require.NoError(t, err)
	}
}

func mkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
}
