package safari

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
)

func testDef() browser.Def {
	p := browser.Platform{UserDataDirs: []string{"Library/Safari"}}
	return browser.Def{ID: "safari", Name: "Safari", Base: browser.Safari, Darwin: p, Linux: p, Windows: p}
}

// libraryAt lays out a fake home: ~/Library/Safari with a History.db, and the container beside it.
func libraryAt(t *testing.T, home string) (container string) {
	t.Helper()
	mkFile(t, home, "Library", "Safari", "History.db")
	return filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library")
}

func discoverProfiles(t *testing.T, home string) []browser.Profile {
	t.Helper()
	dirs, err := Discover(testDef(), browser.DiscoverOptions{Home: home})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, browser.DirID("safari"), dirs[0].ID)
	assert.Equal(t, filepath.Join(home, "Library"), dirs[0].Path)
	return dirs[0].Profiles
}

func named(name, uuid string) browser.Profile {
	return browser.Profile{Name: name, Dir: namedProfilesDir + "/" + uuid}
}

var defaultProfile = browser.Profile{Name: "default", Dir: "Safari"}

func TestDiscover_DefaultOnly(t *testing.T) {
	home := t.TempDir()
	libraryAt(t, home)
	assert.Equal(t, []browser.Profile{defaultProfile}, discoverProfiles(t, home))
}

func TestDiscover_MissingSafariYieldsNothing(t *testing.T) {
	dirs, err := Discover(testDef(), browser.DiscoverOptions{Home: t.TempDir()})
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// -p does not apply to Safari: the table's directory is used, and Validate rejects -p at the switch-over.
func TestDiscover_IgnoresProfilePath(t *testing.T) {
	home := t.TempDir()
	libraryAt(t, home)
	dirs, err := Discover(testDef(), browser.DiscoverOptions{Home: home, ProfilePath: t.TempDir()})
	require.NoError(t, err)
	require.Len(t, dirs, 1)
	assert.Equal(t, filepath.Join(home, "Library"), dirs[0].Path)
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

// A '#' in the live path cuts a SQLite URI short, so the titles come through only when discovery opens its copy.
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

// A profile directory SafariTabs.db does not mention stays hidden: a readable DB is authoritative.
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

// Without SafariTabs.db the Profiles directory is listed and names are synthesized; directories without data are kept,
// because whether a category's file exists is the plan's question.
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
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveProfileName(tt.title, tt.uuid))
		})
	}
}
