package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
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

	var out testCollector
	err := extractHistories(path, &out)
	got := collected[record.History](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Rows keep the database's order; the exporter sorts them.
	assert.Equal(t, 100, got[0].VisitCount)
	assert.Equal(t, 50, got[1].VisitCount)
	assert.Equal(t, 200, got[2].VisitCount)

	// Verify field mapping
	assert.Equal(t, "https://example.com", got[2].URL)
	assert.Equal(t, "Example", got[2].Title)
	assert.False(t, got[2].LastVisit.IsZero())
}

func TestExtractHistories_FileNotFound(t *testing.T) {
	err := extractHistories("/nonexistent/History", &testCollector{})
	require.Error(t, err)
}
