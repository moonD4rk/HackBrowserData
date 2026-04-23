package safari

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSafariHistoryDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "History.db",
		[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
		insertHistoryItem(1, "https://github.com", "github.com", 100),
		insertHistoryItem(2, "https://go.dev", "go.dev", 50),
		insertHistoryItem(3, "https://example.com", "example.com", 200),
		// Item 1 has two visits — extractHistories must deduplicate.
		insertHistoryVisit(1, 1, 704067600.0, "GitHub"),
		insertHistoryVisit(2, 1, 705067600.0, "GitHub - Latest"),
		insertHistoryVisit(3, 2, 703067600.0, "The Go Programming Language"),
		insertHistoryVisit(4, 3, 700067600.0, "Example Domain"),
	)
}

func TestExtractHistories(t *testing.T) {
	path := setupSafariHistoryDB(t)

	got, err := extractHistories(path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Sorted by visit count descending (most visited first).
	assert.Equal(t, 200, got[0].VisitCount)
	assert.Equal(t, 100, got[1].VisitCount)
	assert.Equal(t, 50, got[2].VisitCount)

	// Verify field mapping.
	assert.Equal(t, "https://example.com", got[0].URL)
	assert.Equal(t, "https://github.com", got[1].URL)
	assert.Equal(t, "https://go.dev", got[2].URL)
	assert.False(t, got[0].LastVisit.IsZero())
}

func TestExtractHistories_Dedup(t *testing.T) {
	path := setupSafariHistoryDB(t)

	got, err := extractHistories(path)
	require.NoError(t, err)
	// 3 history_items, not 4 visits.
	require.Len(t, got, 3)

	// GitHub (item 1) should have the later visit_time and its title.
	for _, h := range got {
		if h.URL == "https://github.com" {
			// 705067600 + 978307200 = 1683374800 (unix)
			assert.Equal(t, int64(1683374800), h.LastVisit.Unix())
			// Title must come from the latest visit row, not an arbitrary one.
			assert.Equal(t, "GitHub - Latest", h.Title)
			return
		}
	}
	t.Fatal("expected https://github.com in results")
}

func TestCountHistories(t *testing.T) {
	path := setupSafariHistoryDB(t)

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountHistories_Empty(t *testing.T) {
	path := createTestDB(t, "History.db",
		[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema})

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestExtractHistories_NullTitle(t *testing.T) {
	path := createTestDB(t, "History.db",
		[]string{safariHistoryItemsSchema, safariHistoryVisitsSchema},
		insertHistoryItem(1, "https://null.test", "null.test", 1),
		// Visit with NULL title — COALESCE should return empty string.
		`INSERT INTO history_visits (id, history_item, visit_time) VALUES (1, 1, 700000000.0)`,
	)

	got, err := extractHistories(path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "https://null.test", got[0].URL)
	assert.Empty(t, got[0].Title)
}

func TestCoredataTimestamp(t *testing.T) {
	// A zero Core Data value is treated as "no timestamp" and returns
	// the zero time.Time rather than literal 2001-01-01 — matches the
	// convention used by the Chromium and Firefox helpers.
	assert.True(t, coredataTimestamp(0).IsZero())

	// Known value: 700000000 Core Data = 1678307200 Unix
	ts2 := coredataTimestamp(700000000)
	assert.Equal(t, int64(1678307200), ts2.Unix())
}
