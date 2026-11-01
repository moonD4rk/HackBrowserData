package sqlitex

import (
	"database/sql"
	"fmt"
	"os"
)

// CountRows fails fast on a scan error, unlike QueryRows, which skips the row.
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

// Rows that fail to scan are skipped and logged at debug level.
func QueryRows[T any](dbPath string, journalOff bool, query string, scanRow func(*sql.Rows) (T, error)) ([]T, error) {
	var items []T
	err := eachRow(dbPath, journalOff, query, func(rows *sql.Rows) error {
		item, err := scanRow(rows)
		if err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
