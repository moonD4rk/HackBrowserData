package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHistories(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
		insertMozPlace(1, "https://github.com", "GitHub", 100, 1700000000000000),
		insertMozPlace(2, "https://go.dev", "Go", 50, 1710000000000000),
		insertMozPlace(3, "https://example.com", "Example", 200, 1690000000000000),
	)

	got, err := extractHistories(path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Verify sort order: visit count ascending (Firefox convention)
	assert.Equal(t, 50, got[0].VisitCount)
	assert.Equal(t, 100, got[1].VisitCount)
	assert.Equal(t, 200, got[2].VisitCount)

	// Verify field mapping (first = least visited)
	assert.Equal(t, "https://go.dev", got[0].URL)
	assert.Equal(t, "Go", got[0].Title)
	assert.False(t, got[0].LastVisit.IsZero())
}

func TestExtractHistories_NullFields(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
		// last_visit_date=NULL, title=NULL — COALESCE should handle
		`INSERT INTO moz_places (id, url, visit_count, rev_host, guid, url_hash)
		 VALUES (1, 'https://null.test', 1, '', 'g1', 0)`,
	)

	got, err := extractHistories(path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "https://null.test", got[0].URL)
	assert.Equal(t, "", got[0].Title)
}
