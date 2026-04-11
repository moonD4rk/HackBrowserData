package safari

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Real Safari table schemas — extracted via `sqlite3 History.db ".schema"`.
// ---------------------------------------------------------------------------

const safariHistoryItemsSchema = `CREATE TABLE history_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT NOT NULL UNIQUE,
	domain_expansion TEXT NULL,
	visit_count INTEGER NOT NULL DEFAULT 0,
	daily_visit_counts BLOB NOT NULL DEFAULT x'',
	weekly_visit_counts BLOB NULL,
	autocomplete_triggers BLOB NULL,
	should_recompute_derived_visit_counts INTEGER NOT NULL DEFAULT 1,
	visit_count_score INTEGER NOT NULL DEFAULT 0
)`

const safariHistoryVisitsSchema = `CREATE TABLE history_visits (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	history_item INTEGER NOT NULL REFERENCES history_items(id),
	visit_time REAL NOT NULL,
	title TEXT NULL,
	load_successful BOOLEAN NOT NULL DEFAULT 1,
	http_non_get INTEGER NOT NULL DEFAULT 0,
	synthesized INTEGER NOT NULL DEFAULT 0,
	redirect_source INTEGER NULL,
	redirect_destination INTEGER NULL,
	origin INTEGER NOT NULL DEFAULT 0,
	generation INTEGER NOT NULL DEFAULT 0,
	attributes INTEGER NOT NULL DEFAULT 0,
	score INTEGER NOT NULL DEFAULT 0
)`

// ---------------------------------------------------------------------------
// INSERT helpers
// ---------------------------------------------------------------------------

func insertHistoryItem(id int, url, domainExpansion string, visitCount int) string {
	return fmt.Sprintf(
		`INSERT INTO history_items (id, url, domain_expansion, visit_count)
		 VALUES (%d, '%s', '%s', %d)`,
		id, url, domainExpansion, visitCount,
	)
}

func insertHistoryVisit(id, historyItem int, visitTime float64, title string) string {
	return fmt.Sprintf(
		`INSERT INTO history_visits (id, history_item, visit_time, title)
		 VALUES (%d, %d, %f, '%s')`,
		id, historyItem, visitTime, title,
	)
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

func createTestDB(t *testing.T, name string, schemas []string, inserts ...string) string { //nolint:unparam // name will vary when future data types are added
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	for _, schema := range schemas {
		_, err = db.Exec(schema)
		require.NoError(t, err)
	}
	for _, stmt := range inserts {
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}
	return path
}
