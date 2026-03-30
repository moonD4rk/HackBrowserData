package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBookmarks(t *testing.T) {
	// Bookmarks require JOIN: moz_bookmarks.fk = moz_places.id
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozBookmarksSchema},
		insertMozPlace(1, "https://go.dev", "Go", 0, 0),
		insertMozPlace(2, "https://github.com", "GitHub", 0, 0),
		insertMozBookmark(1, 1, 1, "Go Website", 1700000000000000),
		insertMozBookmark(2, 2, 1, "GitHub", 1710000000000000),
	)

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
