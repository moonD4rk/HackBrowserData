package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMozBookmarkDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozBookmarksSchema},
		insertMozPlace(1, "https://go.dev", "Go", 0, 0),
		insertMozPlace(2, "https://github.com", "GitHub", 0, 0),
		insertMozBookmark(1, 1, 1, "Go Website", 1700000000000000),
		insertMozBookmark(2, 2, 1, "GitHub", 1710000000000000),
	)
}

func TestExtractBookmarks(t *testing.T) {
	path := setupMozBookmarkDB(t)

	got, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: dateAdded descending
	assert.Equal(t, "GitHub", got[0].Name)
	assert.Equal(t, "Go Website", got[1].Name)

	// Verify field mapping
	assert.Equal(t, "https://github.com", got[0].URL)
	assert.Equal(t, "url", got[0].Folder) // type=1 → "url"
	assert.False(t, got[0].CreatedAt.IsZero())
}

func TestCountBookmarks(t *testing.T) {
	path := setupMozBookmarkDB(t)

	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountBookmarks_Empty(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozBookmarksSchema})

	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
