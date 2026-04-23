package firefox

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

// Shared fixtures built once for all tests.
var fixture struct {
	root       string
	multiProf  string // two Firefox profiles + non-profile dir
	singleProf string // one profile with all data files
	partial    string // profile missing some files
	empty      string
}

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "firefox-test-*")
	if err != nil {
		panic(err)
	}
	fixture.root = root
	buildFixtures()
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

func buildFixtures() {
	allFiles := []string{
		"places.sqlite", "cookies.sqlite", "logins.json",
		"extensions.json", "webappsstore.sqlite", "key4.db",
	}

	// Multi-profile: two valid profiles + one non-profile directory
	fixture.multiProf = filepath.Join(fixture.root, "multi")
	for _, prof := range []string{"abc123.default-release", "xyz789.default"} {
		for _, f := range allFiles {
			mkFile(fixture.multiProf, prof, f)
		}
	}
	mkDir(fixture.multiProf, "Crash Reports")
	mkDir(fixture.multiProf, "Pending Pings")

	// Single profile: one profile with all files
	fixture.singleProf = filepath.Join(fixture.root, "single")
	for _, f := range allFiles {
		mkFile(fixture.singleProf, "m1n2o3.default-release", f)
	}

	// Partial profile: only places.sqlite (no logins, no cookies)
	fixture.partial = filepath.Join(fixture.root, "partial")
	mkFile(fixture.partial, "p4q5r6.default", "places.sqlite")

	// Empty directory
	fixture.empty = filepath.Join(fixture.root, "empty")
	mkDir(fixture.empty)
}

func mkFile(parts ...string) {
	path := filepath.Join(parts...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		panic(err)
	}
}

func mkDir(parts ...string) {
	if err := os.MkdirAll(filepath.Join(parts...), 0o755); err != nil {
		panic(err)
	}
}

// TestNewBrowsers is table-driven, covering all profile discovery scenarios.
func TestNewBrowsers(t *testing.T) {
	tests := []struct {
		name         string
		dir          string
		wantProfiles []string // expected profile base names
		skipDirs     []string // should NOT appear as profiles
	}{
		{
			name:         "multi-profile discovery",
			dir:          fixture.multiProf,
			wantProfiles: []string{"abc123.default-release", "xyz789.default"},
			skipDirs:     []string{"Crash Reports", "Pending Pings"},
		},
		{
			name:         "single profile",
			dir:          fixture.singleProf,
			wantProfiles: []string{"m1n2o3.default-release"},
		},
		{
			name:         "partial profile",
			dir:          fixture.partial,
			wantProfiles: []string{"p4q5r6.default"},
		},
		{
			name: "empty dir",
			dir:  fixture.empty,
		},
		{
			name: "nonexistent dir",
			dir:  "/nonexistent/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := types.BrowserConfig{Name: "Firefox", Kind: types.Firefox, UserDataDir: tt.dir}
			browsers, err := NewBrowsers(cfg)
			require.NoError(t, err)

			if len(tt.wantProfiles) == 0 {
				assert.Empty(t, browsers)
				return
			}
			require.Len(t, browsers, len(tt.wantProfiles))

			profileNames := make(map[string]bool)
			for _, b := range browsers {
				profileNames[filepath.Base(b.profileDir)] = true
			}
			for _, want := range tt.wantProfiles {
				assert.True(t, profileNames[want], "should find profile %s", want)
			}
			for _, skip := range tt.skipDirs {
				assert.False(t, profileNames[skip], "should not find %s", skip)
			}
		})
	}
}

// TestResolveSourcePaths verifies that source resolution correctly maps
// categories to files, including shared files (places.sqlite).
func TestResolveSourcePaths(t *testing.T) {
	profileDir := filepath.Join(fixture.singleProf, "m1n2o3.default-release")
	resolved := resolveSourcePaths(firefoxSources, profileDir)

	// All categories should be resolved
	for _, cat := range []types.Category{
		types.Password, types.Cookie, types.History,
		types.Download, types.Bookmark, types.Extension, types.LocalStorage,
	} {
		assert.Contains(t, resolved, cat, "should resolve %s", cat)
	}

	// History, Download, Bookmark share places.sqlite
	assert.Equal(t, resolved[types.History].absPath, resolved[types.Download].absPath)
	assert.Equal(t, resolved[types.History].absPath, resolved[types.Bookmark].absPath)

	// Password is a different file
	assert.NotEqual(t, resolved[types.Password].absPath, resolved[types.History].absPath)
}

func TestResolveSourcePaths_Partial(t *testing.T) {
	profileDir := filepath.Join(fixture.partial, "p4q5r6.default")
	resolved := resolveSourcePaths(firefoxSources, profileDir)

	// Only places.sqlite exists → History, Download, Bookmark resolved
	assert.Contains(t, resolved, types.History)
	assert.Contains(t, resolved, types.Download)
	assert.Contains(t, resolved, types.Bookmark)

	// No logins.json, cookies.sqlite, etc.
	assert.NotContains(t, resolved, types.Password)
	assert.NotContains(t, resolved, types.Cookie)
	assert.NotContains(t, resolved, types.Extension)
}

// ---------------------------------------------------------------------------
// CountEntries
// ---------------------------------------------------------------------------

func TestCountEntries(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "test-profile")
	mkDir(profileDir)
	installFile(t, profileDir, setupMozHistoryDB(t), "places.sqlite")

	browsers, err := NewBrowsers(types.BrowserConfig{
		Name: "Firefox", Kind: types.Firefox, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.Len(t, browsers, 1)

	// CountEntries works without master key.
	counts, err := browsers[0].CountEntries([]types.Category{types.History})
	require.NoError(t, err)
	assert.Equal(t, 3, counts[types.History])
}

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := setupMozHistoryDB(t)
		b := &Browser{}
		assert.Equal(t, 3, b.countCategory(types.History, path))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := setupMozCookieDB(t)
		b := &Browser{}
		assert.Equal(t, 2, b.countCategory(types.Cookie, path))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := setupMozBookmarkDB(t)
		b := &Browser{}
		assert.Equal(t, 2, b.countCategory(types.Bookmark, path))
	})

	t.Run("Extension", func(t *testing.T) {
		path := setupMozExtensionJSON(t)
		b := &Browser{}
		assert.Equal(t, 2, b.countCategory(types.Extension, path))
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		b := &Browser{}
		assert.Equal(t, 0, b.countCategory(types.CreditCard, "unused"))
		assert.Equal(t, 0, b.countCategory(types.SessionStorage, "unused"))
	})
}

// ---------------------------------------------------------------------------
// extractCategory
// ---------------------------------------------------------------------------

// TestExtractCategory verifies that the switch dispatch works for each category.
func TestExtractCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := createTestDB(t, "places.sqlite",
			[]string{mozPlacesSchema},
			insertMozPlace(1, "https://example.com", "Example", 3, 1000000),
			insertMozPlace(2, "https://go.dev", "Go", 1, 2000000),
		)
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.History, nil, path)

		require.Len(t, data.Histories, 2)
		// Firefox sorts by visit count ascending
		assert.Equal(t, 1, data.Histories[0].VisitCount)
		assert.Equal(t, 3, data.Histories[1].VisitCount)
	})

	t.Run("Cookie", func(t *testing.T) {
		path := createTestDB(t, "cookies.sqlite",
			[]string{mozCookiesSchema},
			insertMozCookie("session", "abc", ".example.com", "/", 1000000000000, 0, 0, 0),
		)
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.Cookie, nil, path)

		require.Len(t, data.Cookies, 1)
		assert.Equal(t, "session", data.Cookies[0].Name)
		assert.Equal(t, "abc", data.Cookies[0].Value) // Firefox cookies are not encrypted
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := createTestDB(t, "places.sqlite",
			[]string{mozPlacesSchema, mozBookmarksSchema},
			insertMozPlace(1, "https://github.com", "GitHub", 1, 1000000),
			insertMozBookmark(1, 1, 1, "GitHub", 1000000),
		)
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.Bookmark, nil, path)

		require.Len(t, data.Bookmarks, 1)
		assert.Equal(t, "GitHub", data.Bookmarks[0].Name)
	})

	t.Run("Extension", func(t *testing.T) {
		path := createTestJSON(t, "extensions.json", `{
			"addons": [
				{
					"id": "ublock@example.com",
					"location": "app-profile",
					"active": true,
					"version": "1.0",
					"defaultLocale": {"name": "uBlock Origin", "description": "Ad blocker"}
				},
				{
					"id": "system@mozilla.com",
					"location": "app-system-defaults",
					"active": true
				}
			]
		}`)
		b := &Browser{}
		data := &types.BrowserData{}
		b.extractCategory(data, types.Extension, nil, path)

		require.Len(t, data.Extensions, 1) // system extension skipped
		assert.Equal(t, "uBlock Origin", data.Extensions[0].Name)
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		b := &Browser{}
		data := &types.BrowserData{}
		// CreditCard and SessionStorage are not supported by Firefox
		b.extractCategory(data, types.CreditCard, nil, "unused")
		b.extractCategory(data, types.SessionStorage, nil, "unused")
		assert.Empty(t, data.CreditCards)
		assert.Empty(t, data.SessionStorage)
	})
}

// Anchor: 2024-01-15T10:30:00Z.
const anchorUnixSeconds = int64(1705314600)

func TestFirefoxMicros_AnchorDate(t *testing.T) {
	got := firefoxMicros(anchorUnixSeconds * 1_000_000)
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

func TestFirefoxMicros_PrecisionPreserved(t *testing.T) {
	got := firefoxMicros(anchorUnixSeconds*1_000_000 + 123456)
	assert.Equal(t, 123456*int64(time.Microsecond), int64(got.Nanosecond()))
}

func TestFirefoxMillis_AnchorDate(t *testing.T) {
	got := firefoxMillis(anchorUnixSeconds * 1_000)
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

func TestFirefoxMillis_PrecisionPreserved(t *testing.T) {
	got := firefoxMillis(anchorUnixSeconds*1_000 + 789)
	assert.Equal(t, 789*int64(time.Millisecond), int64(got.Nanosecond()))
}

func TestFirefoxSeconds_AnchorDate(t *testing.T) {
	got := firefoxSeconds(anchorUnixSeconds)
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

func TestFirefoxHelpers_ZeroReturnsZeroTime(t *testing.T) {
	assert.True(t, firefoxMicros(0).IsZero(), "micros")
	assert.True(t, firefoxMillis(0).IsZero(), "millis")
	assert.True(t, firefoxSeconds(0).IsZero(), "seconds")
}

func TestFirefoxHelpers_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, firefoxMicros(-1).IsZero(), "micros")
	assert.True(t, firefoxMillis(-1).IsZero(), "millis")
	assert.True(t, firefoxSeconds(-1).IsZero(), "seconds")
}

func TestFirefoxHelpers_AlwaysUTC(t *testing.T) {
	// assert.Same: pointer equality reliably catches any helper that
	// leaks time.Local, independent of the runner's configured TZ.
	assert.Same(t, time.UTC, firefoxMicros(anchorUnixSeconds*1_000_000).Location())
	assert.Same(t, time.UTC, firefoxMillis(anchorUnixSeconds*1_000).Location())
	assert.Same(t, time.UTC, firefoxSeconds(anchorUnixSeconds).Location())
}

func TestFirefoxHelpers_SameMomentAcrossUnits(t *testing.T) {
	us := firefoxMicros(anchorUnixSeconds * 1_000_000)
	ms := firefoxMillis(anchorUnixSeconds * 1_000)
	s := firefoxSeconds(anchorUnixSeconds)
	assert.True(t, us.Equal(ms))
	assert.True(t, ms.Equal(s))
}

func TestFirefoxHelpers_OutOfJSONRangeReturnsZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  time.Time
	}{
		{"seconds", firefoxSeconds(1 << 50)},
		{"millis", firefoxMillis(1 << 60)},
		{"micros", firefoxMicros(1 << 62)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.got.MarshalJSON()
			require.NoError(t, err)
			assert.JSONEq(t, `"0001-01-01T00:00:00Z"`, string(b))
		})
	}
}
