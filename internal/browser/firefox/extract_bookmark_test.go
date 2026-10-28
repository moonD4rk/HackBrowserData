package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
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

	var out testCollector
	err := extractBookmarks(path, &out)
	got := collected[record.Bookmark](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Rows keep the database's order; the exporter sorts them.
	assert.Equal(t, "Go Website", got[0].Name)
	assert.Equal(t, "GitHub", got[1].Name)

	// Verify field mapping
	assert.Equal(t, "https://github.com", got[1].URL)
	assert.Equal(t, "url", got[1].Folder) // type=1 → "url"
	assert.False(t, got[1].CreatedAt.IsZero())
}
