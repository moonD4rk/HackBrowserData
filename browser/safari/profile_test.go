package safari

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

const fixtureWorkUUID = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"

func writePlist(t *testing.T, path string, v any) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, plist.NewBinaryEncoder(f).Encode(v))
}

func installDB(t *testing.T, dst, src string) {
	t.Helper()
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	require.NoError(t, os.WriteFile(dst, data, 0o644))
}

// libraryFixture is a ~/Library with a default and a named profile: bookmarks belong to the default one only, and the
// shared Downloads.plist holds one entry for each.
func libraryFixture(t *testing.T) string {
	t.Helper()
	library := t.TempDir()
	legacyHome := filepath.Join(library, "Safari")
	container := filepath.Join(library, "Containers", "com.apple.Safari", "Data", "Library")

	writeSafariTabsDB(t, filepath.Join(container, "Safari", "SafariTabs.db"), []tabRow{{uuid: fixtureWorkUUID, title: "work"}})
	installDB(t, filepath.Join(legacyHome, "History.db"), createTestDB(t, "History.db",
		[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
		insertHistoryItem(1, "https://home.test/a", "home.test", 3),
		insertHistoryItem(2, "https://home.test/b", "home.test", 1),
	))
	installDB(t, filepath.Join(container, "Safari", "Profiles", fixtureWorkUUID, "History.db"), createTestDB(t, "History.db",
		[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
		insertHistoryItem(1, "https://work.test/", "work.test", 5),
	))
	writePlist(t, filepath.Join(legacyHome, "Bookmarks.plist"), map[string]any{
		"WebBookmarkType": "WebBookmarkTypeList",
		"Children": []any{map[string]any{
			"WebBookmarkType": "WebBookmarkTypeList",
			"Title":           "BookmarksBar",
			"Children": []any{map[string]any{
				"WebBookmarkType": "WebBookmarkTypeLeaf",
				"URLString":       "https://go.dev/",
				"URIDictionary":   map[string]any{"title": "Go"},
			}},
		}},
	})
	writePlist(t, filepath.Join(legacyHome, "Downloads.plist"), map[string]any{
		"DownloadHistory": []any{
			map[string]any{"DownloadEntryURL": "https://home.test/a.zip", "DownloadEntryPath": "/tmp/a.zip", "DownloadEntryProfileUUIDStringKey": "DefaultProfile"},
			map[string]any{"DownloadEntryURL": "https://work.test/b.zip", "DownloadEntryPath": "/tmp/b.zip", "DownloadEntryProfileUUIDStringKey": fixtureWorkUUID},
		},
	})
	return legacyHome
}

func fixtureBrowser(t *testing.T) *Browser {
	t.Helper()
	b, err := NewBrowser(types.BrowserConfig{Name: "Safari", Kind: types.Safari, UserDataDir: libraryFixture(t)})
	require.NoError(t, err)
	require.NotNil(t, b)
	return b
}

// Password is left out: it would read this machine's login keychain.
var adapterCategories = []category.Category{category.History, category.Bookmark, category.Download, category.Cookie, category.Extension}

func TestExtract_RunsTheBaseOnEachProfile(t *testing.T) {
	results, err := fixtureBrowser(t).Extract(adapterCategories)
	require.NoError(t, err)
	require.Len(t, results, 2)

	byName := map[string]*types.BrowserData{}
	for _, r := range results {
		byName[r.Name] = r.Data
	}
	require.Contains(t, byName, "default")
	require.Contains(t, byName, "work")
	assert.Len(t, byName["default"].Histories, 2)
	assert.Len(t, byName["default"].Bookmarks, 1)
	require.Len(t, byName["default"].Downloads, 1)
	assert.Equal(t, "https://home.test/a.zip", byName["default"].Downloads[0].URL)
	assert.Len(t, byName["work"].Histories, 1)
	assert.Empty(t, byName["work"].Bookmarks, "Bookmarks.plist belongs to the default profile")
	require.Len(t, byName["work"].Downloads, 1)
	assert.Equal(t, "https://work.test/b.zip", byName["work"].Downloads[0].URL)
}

func TestCountEntries_MatchesExtract(t *testing.T) {
	results, err := fixtureBrowser(t).CountEntries(adapterCategories)
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, r := range results {
		switch r.Name {
		case "default":
			assert.Equal(t, 2, r.Counts[category.History])
			assert.Equal(t, 1, r.Counts[category.Bookmark])
			assert.Equal(t, 1, r.Counts[category.Download])
		case "work":
			assert.Equal(t, 1, r.Counts[category.History])
			assert.Equal(t, 0, r.Counts[category.Bookmark])
			assert.Equal(t, 1, r.Counts[category.Download])
		default:
			t.Fatalf("unexpected profile %q", r.Name)
		}
	}
}
