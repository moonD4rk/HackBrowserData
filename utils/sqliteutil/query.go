package sqliteutil

import (
	"database/sql"
	"fmt"
)

// CountRows runs a scalar count query (e.g. SELECT COUNT(*) FROM ...) and
// returns the integer result. It reuses the same database-open logic as
// QuerySQLite (file existence check, optional journal_mode=off).
func CountRows(dbPath string, journalOff bool, query string) (int, error) {
	var count int
	err := QuerySQLite(dbPath, journalOff, query, func(rows *sql.Rows) error {
		return rows.Scan(&count)
	})
	if err != nil {
		return 0, fmt.Errorf("count rows: %w", err)
	}
	return count, nil
}

// QueryRows is a generic helper (Go 1.18+) that wraps QuerySQLite and collects
// results into a typed slice. Each extract method only needs to provide the
// scan function that converts one database row into a typed value.
//
// Rows that fail to scan are skipped (logged at debug level by QuerySQLite).
func QueryRows[T any](dbPath string, journalOff bool, query string, scanRow func(*sql.Rows) (T, error)) ([]T, error) {
	var items []T
	err := QuerySQLite(dbPath, journalOff, query, func(rows *sql.Rows) error {
		item, err := scanRow(rows)
		if err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
