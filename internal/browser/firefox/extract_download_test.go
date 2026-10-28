package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func setupMozDownloadDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozAnnosSchema},
		insertMozPlace(1, "https://example.com/old.zip", "Old File", 0, 0),
		insertMozPlace(2, "https://example.com/new.pdf", "New File", 0, 0),
		insertMozAnno(1, "/tmp/old.zip", 1700000000000000),
		insertMozAnno(2, "/tmp/new.pdf", 1710000000000000),
	)
}

func TestExtractDownloads(t *testing.T) {
	path := setupMozDownloadDB(t)

	var out testCollector
	err := extractDownloads(path, &out)
	got := collected[record.Download](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Rows keep the database's order; the exporter sorts them.
	assert.Equal(t, "https://example.com/old.zip", got[0].URL)
	assert.Equal(t, "https://example.com/new.pdf", got[1].URL)

	// Verify field mapping
	assert.Equal(t, "/tmp/new.pdf", got[1].TargetPath)
	assert.False(t, got[1].StartTime.IsZero())
}
