package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHistoryDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "History", urlsSchema,
		insertURL("https://github.com", "GitHub", 100, 13370000000000000),
		insertURL("https://go.dev", "Go Dev", 50, 13360000000000000),
		insertURL("https://example.com", "Example", 200, 13350000000000000),
	)
}

func TestExtractHistories(t *testing.T) {
	path := setupHistoryDB(t)

	got, err := extractHistories(path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Verify sort order: visit count descending
	assert.Equal(t, 200, got[0].VisitCount)
	assert.Equal(t, 100, got[1].VisitCount)
	assert.Equal(t, 50, got[2].VisitCount)

	// Verify field mapping
	assert.Equal(t, "https://example.com", got[0].URL)
	assert.Equal(t, "Example", got[0].Title)
	assert.False(t, got[0].LastVisit.IsZero())
}

func TestCountHistories(t *testing.T) {
	path := setupHistoryDB(t)

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountHistories_Empty(t *testing.T) {
	path := createTestDB(t, "History", urlsSchema)

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestExtractHistories_FileNotFound(t *testing.T) {
	_, err := extractHistories("/nonexistent/History")
	require.Error(t, err)
}
