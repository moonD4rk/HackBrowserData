package sqliteutil

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuerySQLite(t *testing.T) {
	// Create a temp SQLite database
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	_, err = db.Exec("CREATE TABLE items (id INTEGER, name TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO items VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma')")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	// Query using our helper
	var names []string
	err = QuerySQLite(dbPath, false, "SELECT name FROM items ORDER BY id", func(rows *sql.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		names = append(names, name)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, names)
}

func TestQuerySQLite_JournalOff(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE t (v TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO t VALUES ('ok')")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	var values []string
	err = QuerySQLite(dbPath, true, "SELECT v FROM t", func(rows *sql.Rows) error {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		values = append(values, v)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, values)
}

func TestQuerySQLite_FileNotFound(t *testing.T) {
	err := QuerySQLite("/nonexistent/path.db", false, "SELECT 1", func(rows *sql.Rows) error {
		return nil
	})
	require.Error(t, err)
}

func TestQuerySQLite_BadQuery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE t (v TEXT)")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = QuerySQLite(dbPath, false, "SELECT nonexistent FROM t", func(rows *sql.Rows) error {
		return nil
	})
	require.Error(t, err)
}

func TestCountRows(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		inserts    string
		journalOff bool
		query      string
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "count rows",
			schema:    "CREATE TABLE items (id INTEGER, name TEXT)",
			inserts:   "INSERT INTO items VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma')",
			query:     "SELECT COUNT(*) FROM items",
			wantCount: 3,
		},
		{
			name:      "empty table",
			schema:    "CREATE TABLE t (v TEXT)",
			query:     "SELECT COUNT(*) FROM t",
			wantCount: 0,
		},
		{
			name:       "journal off",
			schema:     "CREATE TABLE t (v TEXT)",
			inserts:    "INSERT INTO t VALUES ('a'), ('b')",
			journalOff: true,
			query:      "SELECT COUNT(*) FROM t",
			wantCount:  2,
		},
		{
			name:    "file not found",
			query:   "SELECT COUNT(*) FROM t",
			wantErr: true,
		},
		{
			name:    "bad query",
			schema:  "CREATE TABLE t (v TEXT)",
			query:   "SELECT COUNT(*) FROM nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dbPath string
			if tt.schema != "" {
				dbPath = filepath.Join(t.TempDir(), "test.db")
				db, err := sql.Open("sqlite", dbPath)
				require.NoError(t, err)
				_, err = db.Exec(tt.schema)
				require.NoError(t, err)
				if tt.inserts != "" {
					_, err = db.Exec(tt.inserts)
					require.NoError(t, err)
				}
				require.NoError(t, db.Close())
			} else {
				dbPath = "/nonexistent/path.db"
			}

			count, err := CountRows(dbPath, tt.journalOff, tt.query)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
		})
	}
}

func TestQueryRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE users (name TEXT, age INTEGER)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO users VALUES ('alice', 30), ('bob', 25)")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	type user struct {
		Name string
		Age  int
	}

	users, err := QueryRows(dbPath, false, "SELECT name, age FROM users ORDER BY name",
		func(rows *sql.Rows) (user, error) {
			var u user
			err := rows.Scan(&u.Name, &u.Age)
			return u, err
		})

	require.NoError(t, err)
	assert.Equal(t, []user{{"alice", 30}, {"bob", 25}}, users)
}

func TestQueryRows_Empty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE empty (v TEXT)")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	results, err := QueryRows(dbPath, false, "SELECT v FROM empty",
		func(rows *sql.Rows) (string, error) {
			var v string
			if err := rows.Scan(&v); err != nil {
				return "", err
			}
			return v, nil
		})

	require.NoError(t, err)
	assert.Nil(t, results)
}
