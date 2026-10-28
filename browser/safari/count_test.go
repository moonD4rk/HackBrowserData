package safari

import (
	"database/sql"
	"path/filepath"
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

func TestCountLocalStorage(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://a.com":      {{Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
		"https://b.com":      {{Key: "k3", Value: "v3"}},
		"https://c.com:8443": {{Key: "k4", Value: "v4"}},
	})
	count, err := countLocalStorage(root)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestCountLocalStorage_DirMissing(t *testing.T) {
	count, err := countLocalStorage(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestCountLocalStorageFile_SkipsNullKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ls.sqlite3")
	writeLocalStorageDB(t, dbPath, []testLocalStorageItem{
		{Key: "k1", Value: "v1"},
		{Key: "k2", Value: "v2"},
	}, true /*addNullKey*/)

	count, err := countLocalStorageFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "NULL keys are excluded from count to match extract's skip rule")
}

func TestCountLocalStorageFile_MissingTable(t *testing.T) {
	// Real Safari has origin dirs with LocalStorage/localstorage.sqlite3 but no ItemTable yet
	// (seen during live verification). countLocalStorageFile must surface the error so the
	// caller can log-and-skip rather than counting 0 silently.
	dbPath := filepath.Join(t.TempDir(), "empty.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = countLocalStorageFile(dbPath)
	require.Error(t, err)
}
