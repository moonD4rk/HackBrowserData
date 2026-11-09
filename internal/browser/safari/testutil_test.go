package safari

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// Real Safari schemas from sqlite3 History.db .schema, so the queries are tested against the columns Safari has.

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

func createTestDB(t *testing.T, name string, schemas []string, inserts ...string) string { //nolint:unparam // name will vary when future data types are added
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer database.Close()

	for _, schema := range schemas {
		_, err = database.Exec(schema)
		require.NoError(t, err)
	}
	for _, statement := range inserts {
		_, err = database.Exec(statement)
		require.NoError(t, err)
	}
	return path
}

type testCollector struct {
	records []record.Record
	fails   []error
}

func (c *testCollector) Add(row record.Record) { c.records = append(c.records, row) }
func (c *testCollector) Fail(err error)        { c.fails = append(c.fails, err) }

func collected[T any](t *testing.T, collector *testCollector) []T {
	t.Helper()
	got := make([]T, 0, len(collector.records))
	for _, row := range collector.records {
		value, ok := any(row).(*T)
		require.Truef(t, ok, "collected a %T, want *%T", row, *new(T))
		got = append(got, *value)
	}
	return got
}
