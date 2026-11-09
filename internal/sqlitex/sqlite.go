// Package sqlitex opens the copied SQLite databases that extractors read.
package sqlitex

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"

	_ "modernc.org/sqlite" // registers the "sqlite" driver for database/sql
)

// Option changes how Open opens a database.
type Option func(*settings)

type settings struct {
	parameters url.Values
	pragmas    []string
}

// ReadOnly opens the file without write access.
func ReadOnly(s *settings) { s.parameters.Set("mode", "ro") }

// Immutable promises SQLite the file never changes, so it takes no lock and reads no journal.
func Immutable(s *settings) { s.parameters.Set("immutable", "1") }

// JournalOff stops SQLite journaling the copy, the mode the Firefox and Safari extractors have always read in.
func JournalOff(s *settings) { s.pragmas = append(s.pragmas, "PRAGMA journal_mode=off") }

// Open checks the file first, so a missing database is an error rather than a new empty one.
func Open(path string, options ...Option) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("database file: %w", err)
	}
	opened := settings{parameters: url.Values{}}
	for _, option := range options {
		option(&opened)
	}
	dataSource := path
	if len(opened.parameters) > 0 {
		dataSource = "file:" + path + "?" + opened.parameters.Encode()
	}
	database, err := sql.Open("sqlite", dataSource)
	if err != nil {
		return nil, err
	}
	for _, pragma := range opened.pragmas {
		if _, err := database.Exec(pragma); err != nil {
			return nil, errors.Join(fmt.Errorf("%s: %w", pragma, err), database.Close())
		}
	}
	return database, nil
}
