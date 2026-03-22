package datautil

import (
	"database/sql"

	// sqlite3 driver for database/sql
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/log"
)

// QuerySQLite opens a SQLite database, optionally disables journal mode (required
// for Firefox databases), runs the query, and calls scanFn for each row.
//
// scanFn should return nil to continue iteration, or an error to skip the current
// row (the error is logged at debug level and iteration continues).
func QuerySQLite(dbPath string, journalOff bool, query string, scanFn func(*sql.Rows) error) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if journalOff {
		if _, err := db.Exec("PRAGMA journal_mode=off"); err != nil {
			return err
		}
	}

	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err := scanFn(rows); err != nil {
			log.Debugf("scan row error: %v", err)
			continue
		}
	}
	return rows.Err()
}
