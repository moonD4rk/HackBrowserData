package safari

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

func mkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
}

// ---------------------------------------------------------------------------
// NewBrowsers — backward-compat (single flat profile)
// ---------------------------------------------------------------------------

func TestNewBrowsers(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantLen int
	}{
		{
			name: "dir with History.db",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				mkFile(t, dir, "History.db")
				return dir
			},
			wantLen: 1,
		},
		{
			name: "empty dir",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantLen: 0,
		},
		{
			name: "nonexistent dir",
			setup: func(t *testing.T) string {
				return "/nonexistent/path"
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			cfg := types.BrowserConfig{Name: "Safari", Kind: types.Safari, UserDataDir: dir}
			b, err := NewBrowser(cfg)
			require.NoError(t, err)

			if tt.wantLen == 0 {
				assert.Nil(t, b)
				return
			}
			require.NotNil(t, b)
			assert.Equal(t, "Safari", b.BrowserName())
			require.Len(t, b.Profiles(), 1)
			assert.Equal(t, "default", b.Profiles()[0].Name)
			assert.Equal(t, dir, b.Profiles()[0].Dir)
		})
	}
}

// ---------------------------------------------------------------------------
// NewBrowsers — multi-profile (macOS 14+ named profiles)
// ---------------------------------------------------------------------------

func TestNewBrowsers_MultiProfile(t *testing.T) {
	const uuid = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	uuidLower := strings.ToLower(uuid)

	// Build a pretend ~/Library that mirrors a macOS 14+ layout.
	library := t.TempDir()
	legacyHome := filepath.Join(library, "Safari")
	container := filepath.Join(library, "Containers", "com.apple.Safari", "Data", "Library")

	// Default profile data in legacyHome.
	mkFile(t, legacyHome, "History.db")
	mkFile(t, legacyHome, "Bookmarks.plist")

	// Named profile data under the container.
	mkFile(t, container, "Safari", "Profiles", uuid, "History.db")

	// Named profile's Origins directory (Safari 17+ nested localStorage root) — must exist
	// for resolveSourcePaths to register it.
	namedOriginsDir := filepath.Join(container, "WebKit", "WebsiteDataStore", uuidLower, "Origins")
	require.NoError(t, os.MkdirAll(namedOriginsDir, 0o755))

	// SafariTabs.db registering the named profile with a human-readable title.
	writeSafariTabsDB(t, filepath.Join(container, safariTabsDBRelPath), []tabRow{
		{uuid: "DefaultProfile", title: ""},
		{uuid: uuid, title: "work"},
	})

	cfg := types.BrowserConfig{Name: "Safari", Kind: types.Safari, UserDataDir: legacyHome}
	b, err := NewBrowser(cfg)
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Len(t, b.profiles, 2)

	names := []string{b.profiles[0].ctx.name, b.profiles[1].ctx.name}
	assert.Contains(t, names, "default")
	assert.Contains(t, names, "work")

	for _, p := range b.profiles {
		switch p.ctx.name {
		case "default":
			assert.Equal(t, legacyHome, p.dir())
			assert.Contains(t, p.sourcePaths, types.History)
			assert.Equal(t, filepath.Join(legacyHome, "History.db"), p.sourcePaths[types.History].absPath)
			// Default profile's LocalStorage root (WebsiteData/Default) isn't created in this fixture,
			// so it won't resolve — which is the point: resolveSourcePaths only registers paths that exist.
			assert.NotContains(t, p.sourcePaths, types.LocalStorage)
		case "work":
			assert.Equal(t, filepath.Join(container, "Safari", "Profiles", uuid), p.dir())
			assert.Contains(t, p.sourcePaths, types.History)
			assert.Equal(t,
				filepath.Join(container, "Safari", "Profiles", uuid, "History.db"),
				p.sourcePaths[types.History].absPath)
			require.Contains(t, p.sourcePaths, types.LocalStorage)
			assert.Equal(t, namedOriginsDir, p.sourcePaths[types.LocalStorage].absPath)
			assert.True(t, p.sourcePaths[types.LocalStorage].isDir)
		}
	}
}

// ---------------------------------------------------------------------------
// resolveSourcePaths
// ---------------------------------------------------------------------------

func TestResolveSourcePaths(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "History.db")

	sources := buildSources(profileContext{legacyHome: dir, container: deriveContainerRoot(dir)})
	resolved := resolveSourcePaths(sources)
	assert.Contains(t, resolved, types.History)
	assert.Equal(t, filepath.Join(dir, "History.db"), resolved[types.History].absPath)
	assert.False(t, resolved[types.History].isDir)
}

func TestResolveSourcePaths_Empty(t *testing.T) {
	dir := t.TempDir()
	sources := buildSources(profileContext{legacyHome: dir, container: deriveContainerRoot(dir)})
	assert.Empty(t, resolveSourcePaths(sources))
}

// ---------------------------------------------------------------------------
// CountEntries
// ---------------------------------------------------------------------------

func TestCountEntries(t *testing.T) {
	dir := t.TempDir()
	dbPath := createTestDB(t, "History.db",
		[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
		insertHistoryItem(1, "https://example.com", "example.com", 5),
		insertHistoryItem(2, "https://go.dev", "go.dev", 10),
		insertHistoryVisit(1, 1, 700000000.0, "Example"),
		insertHistoryVisit(2, 2, 700000000.0, "Go"),
	)
	data, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "History.db"), data, 0o644))

	b, err := NewBrowser(types.BrowserConfig{
		Name: "Safari", Kind: types.Safari, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	results, err := b.CountEntries([]types.Category{types.History})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 2, results[0].Counts[types.History])
}

// Anchor: 2024-01-15T10:30:00Z, in seconds past the Core Data epoch (2001-01-01Z).
const anchorCoreDataSeconds = 1705314600 - 978307200

func TestCoredataTimestamp_AnchorDate(t *testing.T) {
	got := coredataTimestamp(float64(anchorCoreDataSeconds))
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

func TestCoredataTimestamp_EpochZero(t *testing.T) {
	assert.True(t, coredataTimestamp(0).IsZero())
}

func TestCoredataTimestamp_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, coredataTimestamp(-1).IsZero())
}

func TestCoredataTimestamp_FractionalSecondsPreserved(t *testing.T) {
	got := coredataTimestamp(float64(anchorCoreDataSeconds) + 0.5)
	assert.Equal(t, 500*int64(time.Millisecond), int64(got.Nanosecond()))
}

func TestCoredataTimestamp_AlwaysUTC(t *testing.T) {
	// assert.Same: pointer equality reliably catches any regression that
	// leaks time.Local, independent of the runner's configured TZ.
	got := coredataTimestamp(float64(anchorCoreDataSeconds))
	assert.Same(t, time.UTC, got.Location())
}
