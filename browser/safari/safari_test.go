package safari

import (
	"os"
	"path/filepath"
	"testing"

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
// NewBrowsers
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
// resolveSourcePaths
// ---------------------------------------------------------------------------

func TestResolveSourcePaths(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "History.db")

	resolved := resolveSourcePaths(safariSources, dir)
	assert.Contains(t, resolved, types.History)
	assert.Equal(t, filepath.Join(dir, "History.db"), resolved[types.History].absPath)
	assert.False(t, resolved[types.History].isDir)
}

func TestResolveSourcePaths_Empty(t *testing.T) {
	resolved := resolveSourcePaths(safariSources, t.TempDir())
	assert.Empty(t, resolved)
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

	t.Run("UnsupportedCategory", func(t *testing.T) {
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.CreditCard, "unused")
		assert.Empty(t, data.CreditCards)
	})
}
