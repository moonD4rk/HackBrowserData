package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
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

// extract runs the Firefox base's extractors on a private snapshot of the profile.
func TestExtract(t *testing.T) {
	profileDir := t.TempDir()
	installFile(t, profileDir, setupMozHistoryDB(t), "places.sqlite")
	installFile(t, profileDir, setupMozCookieDB(t), "cookies.sqlite")
	installFile(t, profileDir, setupMozExtensionJSON(t), "extensions.json")
	p := &profile{profileDir: profileDir, browserName: "Firefox", sourcePaths: resolveSourcePaths(firefoxSources, profileDir)}

	data := p.extract([]category.Category{
		category.History, category.Cookie, category.Extension, category.CreditCard, category.SessionStorage,
	})
	require.Len(t, data.Histories, 3)
	visits := []int{data.Histories[0].VisitCount, data.Histories[1].VisitCount, data.Histories[2].VisitCount}
	assert.ElementsMatch(t, []int{100, 50, 200}, visits)
	require.Len(t, data.Cookies, 2)
	assert.Equal(t, "abc123", data.Cookies[0].Value) // Firefox cookies are not encrypted
	assert.Len(t, data.Extensions, 2)                // the system add-on is skipped
	assert.Empty(t, data.CreditCards)
	assert.Empty(t, data.SessionStorage)
}
