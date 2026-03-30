package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDownloads(t *testing.T) {
	// Downloads require JOIN: moz_annos.place_id = moz_places.id
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozAnnosSchema},
		insertMozPlace(1, "https://example.com/old.zip", "Old File", 0, 0),
		insertMozPlace(2, "https://example.com/new.pdf", "New File", 0, 0),
		insertMozAnno(1, "/tmp/old.zip", 1700000000000000),
		insertMozAnno(2, "/tmp/new.pdf", 1710000000000000),
	)

	got, err := extractDownloads(path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: StartTime descending
	assert.Equal(t, "https://example.com/new.pdf", got[0].URL)
	assert.Equal(t, "https://example.com/old.zip", got[1].URL)

	// Verify field mapping
	assert.Equal(t, "/tmp/new.pdf", got[0].TargetPath)
	assert.False(t, got[0].StartTime.IsZero())
}
