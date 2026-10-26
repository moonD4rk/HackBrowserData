package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := setupMozHistoryDB(t)
		p := &profile{}
		assert.Equal(t, 3, p.countCategory(category.History, path))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := setupMozCookieDB(t)
		p := &profile{}
		assert.Equal(t, 2, p.countCategory(category.Cookie, path))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := setupMozBookmarkDB(t)
		p := &profile{}
		assert.Equal(t, 2, p.countCategory(category.Bookmark, path))
	})

	t.Run("Extension", func(t *testing.T) {
		path := setupMozExtensionJSON(t)
		p := &profile{}
		assert.Equal(t, 2, p.countCategory(category.Extension, path))
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		p := &profile{}
		assert.Equal(t, 0, p.countCategory(category.CreditCard, "unused"))
		assert.Equal(t, 0, p.countCategory(category.SessionStorage, "unused"))
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
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.History, nil, path)

		require.Len(t, data.Histories, 2)
		assert.ElementsMatch(t, []int{3, 1}, []int{data.Histories[0].VisitCount, data.Histories[1].VisitCount})
	})

	t.Run("Cookie", func(t *testing.T) {
		path := createTestDB(t, "cookies.sqlite",
			[]string{mozCookiesSchema},
			insertMozCookie("session", "abc", ".example.com", "/", 1000000000000, 0, 0, 0),
		)
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.Cookie, nil, path)

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
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.Bookmark, nil, path)

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
		p := &profile{}
		data := &types.BrowserData{}
		p.extractCategory(data, category.Extension, nil, path)

		require.Len(t, data.Extensions, 1) // system extension skipped
		assert.Equal(t, "uBlock Origin", data.Extensions[0].Name)
	})

	t.Run("UnsupportedCategory", func(t *testing.T) {
		p := &profile{}
		data := &types.BrowserData{}
		// CreditCard and SessionStorage are not supported by Firefox
		p.extractCategory(data, category.CreditCard, nil, "unused")
		p.extractCategory(data, category.SessionStorage, nil, "unused")
		assert.Empty(t, data.CreditCards)
		assert.Empty(t, data.SessionStorage)
	})
}
