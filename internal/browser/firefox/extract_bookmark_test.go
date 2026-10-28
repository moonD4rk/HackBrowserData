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
	assert.Equal(t, int64(2), got[1].ID)
	assert.Equal(t, "url", got[1].Type)
	assert.Empty(t, got[1].Folder, "this fixture's bookmarks have no parent row")
	assert.False(t, got[1].CreatedAt.IsZero())
}

// Before, every bookmark came out with folder "url", an empty type, and id 0.
func TestExtractBookmarks_FolderTypeAndIDFromTheTree(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozBookmarksSchema},
		insertMozPlace(1, "https://go.dev", "Go", 0, 0),
		insertMozPlace(2, "https://github.com", "GitHub", 0, 0),
		`INSERT INTO moz_bookmarks (id, type, fk, parent, position, title, dateAdded, guid) VALUES
			(1, 2, NULL, 0, 0, '', 1, 'root________'),
			(3, 2, NULL, 1, 1, 'toolbar', 1, 'toolbar_____'),
			(7, 1, 1, 3, 0, 'Go', 1710000000000000, 'bookmark0007'),
			(8, 2, NULL, 3, 1, 'Reading', 1, 'folder000008'),
			(9, 1, 2, 8, 0, 'GitHub', 1700000000000000, 'bookmark0009'),
			(10, 3, NULL, 3, 2, NULL, 1, 'separator010')`,
	)

	var out testCollector
	require.NoError(t, extractBookmarks(path, &out))
	byID := map[int64]record.Bookmark{}
	for _, b := range collected[record.Bookmark](t, &out) {
		byID[b.ID] = b
	}
	require.Len(t, byID, 2, "folders and separators are not bookmarks")
	want := record.Bookmark{ID: 7, Name: "Go", Type: "url", URL: "https://go.dev", Folder: "toolbar", CreatedAt: byID[7].CreatedAt}
	assert.Equal(t, want, byID[7])
	assert.Equal(t, "Reading", byID[9].Folder)
	assert.Equal(t, "url", byID[9].Type)
}
