package sqlitex

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createDB(t *testing.T, statements ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	for _, statement := range statements {
		_, err := database.Exec(statement)
		require.NoError(t, err, statement)
	}
	require.NoError(t, database.Close())
	return path
}

func scanNames(names *[]string) func(*sql.Rows) error {
	return func(rows *sql.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		*names = append(*names, name)
		return nil
	}
}

func noSkip(t *testing.T) func(error) {
	return func(err error) { t.Errorf("unexpected skipped row: %v", err) }
}

func TestEachRow(t *testing.T) {
	path := createDB(t,
		"CREATE TABLE items (id INTEGER, name TEXT)",
		"INSERT INTO items VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma')")

	var names []string
	require.NoError(t, EachRow(path, "SELECT name FROM items ORDER BY id", scanNames(&names), noSkip(t)))
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, names)
}

func TestEachRow_JournalOff(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)", "INSERT INTO t VALUES ('x')")

	var names []string
	require.NoError(t, EachRow(path, "SELECT v FROM t", scanNames(&names), noSkip(t), JournalOff))
	assert.Equal(t, []string{"x"}, names)
}

func TestEachRow_ReadOnlyImmutable(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)", "INSERT INTO t VALUES ('x')")

	var names []string
	require.NoError(t, EachRow(path, "SELECT v FROM t", scanNames(&names), noSkip(t), ReadOnly, Immutable))
	assert.Equal(t, []string{"x"}, names)
}

// TestEachRow_SkipsARowThatFailsToScan hands the failure to skip, where an extractor reports it as a record problem.
func TestEachRow_SkipsARowThatFailsToScan(t *testing.T) {
	path := createDB(t,
		"CREATE TABLE items (id INTEGER, name TEXT)",
		"INSERT INTO items VALUES (1, 'alpha'), (2, NULL), (3, 'gamma')")

	var (
		names   []string
		skipped []error
	)
	err := EachRow(path, "SELECT name FROM items ORDER BY id", scanNames(&names), func(err error) { skipped = append(skipped, err) })
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "gamma"}, names)
	assert.Len(t, skipped, 1)
}

func TestEachRow_FileNotFound(t *testing.T) {
	err := EachRow(filepath.Join(t.TempDir(), "missing.db"), "SELECT 1", scanNames(new([]string)), noSkip(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database file")
}

func TestEachRow_BadQuery(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)")
	require.Error(t, EachRow(path, "SELECT nonexistent FROM t", scanNames(new([]string)), noSkip(t)))
}

func TestEachRow_ScanErrorStopsNothing(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)", "INSERT INTO t VALUES ('a'), ('b')")
	failure := errors.New("bad row")
	var skipped []error
	err := EachRow(path, "SELECT v FROM t", func(*sql.Rows) error { return failure }, func(err error) { skipped = append(skipped, err) })
	require.NoError(t, err)
	assert.Equal(t, []error{failure, failure}, skipped)
}

func TestOpen_MissingFileCreatesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	_, err := Open(path)
	require.Error(t, err)
	assert.NoFileExists(t, path)
}
