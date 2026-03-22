package sqliteutil

import "database/sql"

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
