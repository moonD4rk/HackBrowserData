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
			browsers, err := NewBrowsers(cfg)
			require.NoError(t, err)

			if tt.wantLen == 0 {
				assert.Empty(t, browsers)
				return
			}
			require.Len(t, browsers, tt.wantLen)
			assert.Equal(t, "Safari", browsers[0].BrowserName())
			assert.Equal(t, "default", browsers[0].ProfileName())
			assert.Equal(t, dir, browsers[0].ProfileDir())
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
	browsers, err := NewBrowsers(cfg)
	require.NoError(t, err)
	require.Len(t, browsers, 2)

	names := []string{browsers[0].ProfileName(), browsers[1].ProfileName()}
	assert.Contains(t, names, "default")
	assert.Contains(t, names, "work")

	for _, b := range browsers {
		switch b.ProfileName() {
		case "default":
			assert.Equal(t, legacyHome, b.ProfileDir())
			assert.Contains(t, b.sourcePaths, types.History)
			assert.Equal(t, filepath.Join(legacyHome, "History.db"), b.sourcePaths[types.History].absPath)
			// Default profile's LocalStorage root (WebsiteData/Default) isn't created in this fixture,
			// so it won't resolve — which is the point: resolveSourcePaths only registers paths that exist.
			assert.NotContains(t, b.sourcePaths, types.LocalStorage)
		case "work":
			assert.Equal(t, filepath.Join(container, "Safari", "Profiles", uuid), b.ProfileDir())
			assert.Contains(t, b.sourcePaths, types.History)
			assert.Equal(t,
				filepath.Join(container, "Safari", "Profiles", uuid, "History.db"),
				b.sourcePaths[types.History].absPath)
			require.Contains(t, b.sourcePaths, types.LocalStorage)
			assert.Equal(t, namedOriginsDir, b.sourcePaths[types.LocalStorage].absPath)
			assert.True(t, b.sourcePaths[types.LocalStorage].isDir)
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

	browsers, err := NewBrowsers(types.BrowserConfig{
		Name: "Safari", Kind: types.Safari, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.Len(t, browsers, 1)

	counts, err := browsers[0].CountEntries([]types.Category{types.History})
	require.NoError(t, err)
	assert.Equal(t, 2, counts[types.History])
}

// ---------------------------------------------------------------------------
// countCategory / extractCategory
// ---------------------------------------------------------------------------

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := createTestDB(t, "History.db",
			[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
			insertHistoryItem(1, "https://example.com", "example.com", 1),
		)
		b := &Browser{}
		assert.Equal(t, 1, b.countCategory(types.History, path))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := buildTestBinaryCookies(t, []testCookie{
			{domain: ".example.com", name: "a", path: "/", value: "1", expires: 2000000000.0, creation: 700000000.0},
			{domain: ".go.dev", name: "b", path: "/", value: "2", expires: 2000000000.0, creation: 700000000.0},
		})
		b := &Browser{}
		assert.Equal(t, 2, b.countCategory(types.Cookie, path))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := buildTestBookmarksPlist(t, safariBookmark{
			Type: bookmarkTypeList,
			Children: []safariBookmark{
				{Type: bookmarkTypeLeaf, URLString: "https://a.com", URIDictionary: uriDictionary{Title: "A"}},
				{Type: bookmarkTypeLeaf, URLString: "https://b.com", URIDictionary: uriDictionary{Title: "B"}},
			},
		})
		b := &Browser{}
		assert.Equal(t, 2, b.countCategory(types.Bookmark, path))
	})

	t.Run("Download", func(t *testing.T) {
		path := buildTestDownloadsPlist(t, safariDownloads{
			DownloadHistory: []safariDownloadEntry{
				{URL: "https://example.com/file.zip", Path: "/tmp/file.zip", TotalBytes: 100},
			},
		})
		b := &Browser{}
		assert.Equal(t, 1, b.countCategory(types.Download, path))
	})

	t.Run("LocalStorage", func(t *testing.T) {
		dir := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
			"https://example.com": {{Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
			"https://go.dev":      {{Key: "theme", Value: "dark"}},
		})
		b := &Browser{}
		assert.Equal(t, 3, b.countCategory(types.LocalStorage, dir))
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		b := &Browser{}
		assert.Equal(t, 0, b.countCategory(types.CreditCard, "unused"))
		assert.Equal(t, 0, b.countCategory(types.SessionStorage, "unused"))
	})
}

func TestExtractCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := createTestDB(t, "History.db",
			[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
			insertHistoryItem(1, "https://example.com", "example.com", 3),
			insertHistoryItem(2, "https://go.dev", "go.dev", 1),
			insertHistoryVisit(1, 1, 700000000.0, "Example"),
			insertHistoryVisit(2, 2, 700000000.0, "Go"),
		)
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.History, path)

		require.Len(t, data.Histories, 2)
		// Sorted by visit count descending
		assert.Equal(t, 3, data.Histories[0].VisitCount)
		assert.Equal(t, 1, data.Histories[1].VisitCount)
	})

	t.Run("Cookie", func(t *testing.T) {
		path := buildTestBinaryCookies(t, []testCookie{
			{
				domain: ".example.com", name: "session", path: "/", value: "abc",
				secure: true, httpOnly: true, expires: 2000000000.0, creation: 700000000.0,
			},
		})
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.Cookie, path)

		require.Len(t, data.Cookies, 1)
		assert.Equal(t, ".example.com", data.Cookies[0].Host)
		assert.Equal(t, "session", data.Cookies[0].Name)
		assert.True(t, data.Cookies[0].IsSecure)
		assert.True(t, data.Cookies[0].IsHTTPOnly)
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := buildTestBookmarksPlist(t, safariBookmark{
			Type: bookmarkTypeList,
			Children: []safariBookmark{
				{Type: bookmarkTypeLeaf, URLString: "https://github.com", URIDictionary: uriDictionary{Title: "GitHub"}},
			},
		})
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.Bookmark, path)

		require.Len(t, data.Bookmarks, 1)
		assert.Equal(t, "GitHub", data.Bookmarks[0].Name)
		assert.Equal(t, "https://github.com", data.Bookmarks[0].URL)
	})

	t.Run("Download", func(t *testing.T) {
		path := buildTestDownloadsPlist(t, safariDownloads{
			DownloadHistory: []safariDownloadEntry{
				{URL: "https://example.com/file.zip", Path: "/tmp/file.zip", TotalBytes: 1024},
			},
		})
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.Download, path)

		require.Len(t, data.Downloads, 1)
		assert.Equal(t, "https://example.com/file.zip", data.Downloads[0].URL)
		assert.Equal(t, int64(1024), data.Downloads[0].TotalBytes)
	})

	t.Run("LocalStorage", func(t *testing.T) {
		dir := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
			"https://github.com": {{Key: "theme", Value: "dark"}},
		})
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.LocalStorage, dir)

		require.Len(t, data.LocalStorage, 1)
		assert.Equal(t, "https://github.com", data.LocalStorage[0].URL)
		assert.Equal(t, "theme", data.LocalStorage[0].Key)
		assert.Equal(t, "dark", data.LocalStorage[0].Value)
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.CreditCard, "unused")
		assert.Empty(t, data.CreditCards)
	})
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
