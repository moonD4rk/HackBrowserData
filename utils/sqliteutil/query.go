package sqliteutil

import (
	"database/sql"
	"fmt"
	"os"
)

// CountRows runs a scalar count query (e.g. SELECT COUNT(*) FROM ...) and
// returns the integer result. Unlike QuerySQLite (which swallows per-row scan
// errors), CountRows uses QueryRow for fail-fast behavior on scan failures.
func CountRows(dbPath string, journalOff bool, query string) (int, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return 0, fmt.Errorf("database file: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if journalOff {
		if _, err := db.Exec("PRAGMA journal_mode=off"); err != nil {
			return 0, err
		}
	}

	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
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
