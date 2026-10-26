package safari

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := createTestDB(t, "History.db",
			[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
			insertHistoryItem(1, "https://example.com", "example.com", 1),
		)
		p := &profile{}
		assert.Equal(t, 1, p.countCategory(category.History, path, ""))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := buildTestBinaryCookies(t, []testCookie{
			{domain: ".example.com", name: "a", path: "/", value: "1", expires: 2000000000.0, creation: 700000000.0},
			{domain: ".go.dev", name: "b", path: "/", value: "2", expires: 2000000000.0, creation: 700000000.0},
		})
		p := &profile{}
		assert.Equal(t, 2, p.countCategory(category.Cookie, path, ""))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := buildTestBookmarksPlist(t, safariBookmark{
			Type: bookmarkTypeList,
			Children: []safariBookmark{
				{Type: bookmarkTypeLeaf, URLString: "https://a.com", URIDictionary: uriDictionary{Title: "A"}},
				{Type: bookmarkTypeLeaf, URLString: "https://b.com", URIDictionary: uriDictionary{Title: "B"}},
			},
		})
		p := &profile{}
		assert.Equal(t, 2, p.countCategory(category.Bookmark, path, ""))
	})

	t.Run("Download", func(t *testing.T) {
		path := buildTestDownloadsPlist(t, safariDownloads{
			DownloadHistory: []safariDownloadEntry{
				{URL: "https://example.com/file.zip", Path: "/tmp/file.zip", TotalBytes: 100},
			},
		})
		p := &profile{}
		assert.Equal(t, 1, p.countCategory(category.Download, path, ""))
	})

	t.Run("LocalStorage", func(t *testing.T) {
		dir := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
			"https://example.com": {{Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
			"https://go.dev":      {{Key: "theme", Value: "dark"}},
		})
		p := &profile{}
		assert.Equal(t, 3, p.countCategory(category.LocalStorage, dir, ""))
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		p := &profile{}
		assert.Equal(t, 0, p.countCategory(category.CreditCard, "unused", ""))
		assert.Equal(t, 0, p.countCategory(category.SessionStorage, "unused", ""))
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
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.History, path, "")

		require.Len(t, data.Histories, 2)
		assert.ElementsMatch(t, []int{3, 1}, []int{data.Histories[0].VisitCount, data.Histories[1].VisitCount})
	})

	t.Run("Cookie", func(t *testing.T) {
		path := buildTestBinaryCookies(t, []testCookie{
			{
				domain: ".example.com", name: "session", path: "/", value: "abc",
				secure: true, httpOnly: true, expires: 2000000000.0, creation: 700000000.0,
			},
		})
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.Cookie, path, "")

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
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.Bookmark, path, "")

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
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.Download, path, "")

		require.Len(t, data.Downloads, 1)
		assert.Equal(t, "https://example.com/file.zip", data.Downloads[0].URL)
		assert.Equal(t, int64(1024), data.Downloads[0].TotalBytes)
	})

	t.Run("LocalStorage", func(t *testing.T) {
		dir := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
			"https://github.com": {{Key: "theme", Value: "dark"}},
		})
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.LocalStorage, dir, "")

		require.Len(t, data.LocalStorage, 1)
		assert.Equal(t, "https://github.com", data.LocalStorage[0].URL)
		assert.Equal(t, "theme", data.LocalStorage[0].Key)
		assert.Equal(t, "dark", data.LocalStorage[0].Value)
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.CreditCard, "unused", "")
		assert.Empty(t, data.CreditCards)
	})
}
