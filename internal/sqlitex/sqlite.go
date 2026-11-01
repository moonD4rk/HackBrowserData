package sqlitex

import (
	"database/sql"
	"fmt"
	"os"

	// sqlite3 driver for database/sql
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// journalOff is for Firefox databases; the file is checked first so sql.Open never creates an empty one.
// A row whose scanFn returns an error is skipped and logged at debug level.
func eachRow(dbPath string, journalOff bool, query string, scanFn func(*sql.Rows) error) error {
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("database file: %w", err)
	}
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
