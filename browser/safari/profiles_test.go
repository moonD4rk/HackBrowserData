package safari

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

// containerPaths returns (legacyHome, container) for a fake ~/Library tree
// anchored at root. Call sites use this to mirror the production layout where
// legacyHome sits next to Containers/.
func containerPaths(root string) (string, string) {
	legacyHome := filepath.Join(root, "Safari")
	container := filepath.Join(root, "Containers", "com.apple.Safari", "Data", "Library")
	return legacyHome, container
}

func TestDiscoverSafariProfiles_DefaultOnly(t *testing.T) {
	library := t.TempDir()
	legacyHome, _ := containerPaths(library)
	mkFile(t, legacyHome, "History.db")

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 1)
	assert.Equal(t, "default", got[0].name)
	assert.Empty(t, got[0].uuidUpper)
	assert.Empty(t, got[0].uuidLower)
}

func TestDiscoverSafariProfiles_WithNamedProfile(t *testing.T) {
	const uuid = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	mkFile(t, container, "Safari", "Profiles", uuid, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: "DefaultProfile", title: ""},
		{uuid: uuid, title: "work"},
	})

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 2)
	assert.Equal(t, "default", got[0].name)
	assert.Equal(t, "work", got[1].name)
	assert.Equal(t, uuid, got[1].uuidUpper)
	assert.Equal(t, strings.ToLower(uuid), got[1].uuidLower)
}

func TestDiscoverSafariProfiles_EmptyTitleFallbackToUUID(t *testing.T) {
	const uuid = "ABCDEF01-2345-6789-ABCD-EF0123456789"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: uuid, title: ""},
	})

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 2)
	assert.Equal(t, "profile-abcdef01", got[1].name)
}

func TestDiscoverSafariProfiles_OrphanUUIDWithoutDBEntry(t *testing.T) {
	// Profile directory with a History.db exists on disk but is absent from
	// SafariTabs.db. When the DB is readable and doesn't mention it, we trust
	// the DB — the orphan stays hidden because production filters profiles
	// with no resolvable data in NewBrowsers anyway. Here we assert discovery
	// returns only what the DB declares.
	const dbUUID = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	const orphanUUID = "11111111-2222-3333-4444-555555555555"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	mkFile(t, container, "Safari", "Profiles", dbUUID, "History.db")
	mkFile(t, container, "Safari", "Profiles", orphanUUID, "AppExtensions", "Extensions.plist")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: dbUUID, title: "declared"},
	})

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 2)
	assert.Equal(t, "default", got[0].name)
	assert.Equal(t, "declared", got[1].name)
}

func TestDiscoverSafariProfiles_EmptyDBIsAuthoritative(t *testing.T) {
	// SafariTabs.db exists and is readable but contains no named-profile rows.
	// A stray Profiles/<UUID>/ directory on disk must NOT sneak in via the
	// ReadDir fallback — the DB is the authoritative source of truth.
	const strayUUID = "99999999-AAAA-BBBB-CCCC-DDDDDDDDDDDD"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	mkFile(t, container, "Safari", "Profiles", strayUUID, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), nil) // zero rows

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 1)
	assert.Equal(t, "default", got[0].name)
}

func TestDiscoverSafariProfiles_MissingDBFallsBackToReadDir(t *testing.T) {
	// SafariTabs.db absent → enumerate Safari/Profiles/ and synthesize names.
	const uuid = "11111111-2222-3333-4444-555555555555"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	mkFile(t, container, "Safari", "Profiles", uuid, "History.db")
	// Deliberately also drop a non-UUID directory that must be ignored.
	require.NoError(t, os.MkdirAll(filepath.Join(container, "Safari", "Profiles", "Bogus"), 0o755))

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 2)
	assert.Equal(t, "default", got[0].name)
	assert.Equal(t, "profile-11111111", got[1].name)
	assert.Equal(t, uuid, got[1].uuidUpper)
}

func TestDiscoverSafariProfiles_DuplicateTitlesDisambiguate(t *testing.T) {
	const uuidA = "AAAAAAAA-0000-0000-0000-000000000001"
	const uuidB = "BBBBBBBB-0000-0000-0000-000000000002"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: uuidA, title: "team"},
		{uuid: uuidB, title: "team"},
	})

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 3)
	// Order mirrors DB insertion order; the second "team" becomes "team-2".
	assert.Equal(t, "default", got[0].name)
	assert.Equal(t, "team", got[1].name)
	assert.Equal(t, "team-2", got[2].name)
}

func TestDiscoverSafariProfiles_UUIDCaseNormalisation(t *testing.T) {
	// SafariTabs.db always stores UUIDs uppercase (verified on real Mac).
	// WebKit/WebsiteDataStore uses lowercase — we must carry both.
	const uuid = "FEDCBA98-7654-3210-FEDC-BA9876543210"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: uuid, title: "alpha"},
	})

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 2)
	assert.Equal(t, uuid, got[1].uuidUpper)
	assert.Equal(t, strings.ToLower(uuid), got[1].uuidLower)
}

func TestDiscoverSafariProfiles_DefaultProfileSentinelIgnored(t *testing.T) {
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: defaultProfileSentinel, title: ""},
	})

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 1)
	assert.Equal(t, "default", got[0].name)
}

func TestDiscoverSafariProfiles_EmptyProfileDirectoryFiltersOutInNewBrowsers(t *testing.T) {
	// Matches the real 4E2D8DD0 orphan on the author's Mac: a profile dir
	// listed in neither SafariTabs.db nor containing any extractable data.
	// Discovery without the DB surfaces it; NewBrowsers then drops it when
	// resolveSourcePaths yields zero matches.
	const uuid = "4E2D8DD0-A7D2-4684-939A-898B7675C700"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	mkFile(t, legacyHome, "History.db")
	mkFile(t, container, "Safari", "Profiles", uuid, "AppExtensions", "Extensions.plist")

	got := discoverSafariProfiles(legacyHome)
	require.Len(t, got, 2) // discovery includes it …
	paths := resolveSourcePaths(buildSources(got[1]))
	assert.Empty(t, paths) // … but no supported data resolves for it.
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

func TestBuildSources_DefaultProfile(t *testing.T) {
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	p := profileContext{legacyHome: legacyHome, container: container}
	sources := buildSources(p)

	assert.Equal(t, filepath.Join(legacyHome, "History.db"), sources[types.History][0].abs)
	assert.Equal(t, filepath.Join(legacyHome, "Bookmarks.plist"), sources[types.Bookmark][0].abs)
	assert.Equal(t, filepath.Join(legacyHome, "Downloads.plist"), sources[types.Download][0].abs)
	require.Len(t, sources[types.Cookie], 2)
	assert.Equal(t, filepath.Join(container, "Cookies", "Cookies.binarycookies"), sources[types.Cookie][0].abs)
	assert.Equal(t, filepath.Join(library, "Cookies", "Cookies.binarycookies"), sources[types.Cookie][1].abs)
}

func TestBuildSources_NamedProfile(t *testing.T) {
	const uuid = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	library := t.TempDir()
	legacyHome, container := containerPaths(library)

	p := profileContext{
		name:       "work",
		uuidUpper:  uuid,
		uuidLower:  strings.ToLower(uuid),
		legacyHome: legacyHome,
		container:  container,
	}
	sources := buildSources(p)

	assert.Equal(t,
		filepath.Join(container, "Safari", "Profiles", uuid, "History.db"),
		sources[types.History][0].abs)
	assert.Equal(t,
		filepath.Join(container, "WebKit", "WebsiteDataStore", strings.ToLower(uuid), "Cookies", "Cookies.binarycookies"),
		sources[types.Cookie][0].abs)
	// Download points at the shared plist — filtering by DownloadEntryProfileUUIDStringKey
	// happens inside extractDownloads, not at the path layer.
	assert.Equal(t, filepath.Join(legacyHome, "Downloads.plist"), sources[types.Download][0].abs)
	// Bookmark is still shared with no per-entry profile tag, so it's attributed to default only.
	assert.NotContains(t, sources, types.Bookmark)
}
