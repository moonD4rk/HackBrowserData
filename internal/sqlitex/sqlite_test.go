package sqlitex

import (
	"database/sql"
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

func readValue(t *testing.T, path string, options ...Option) string {
	t.Helper()
	database, err := Open(path, options...)
	require.NoError(t, err)
	defer database.Close()

	var value string
	require.NoError(t, database.QueryRow("SELECT v FROM t").Scan(&value))
	return value
}

func TestOpen(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)", "INSERT INTO t VALUES ('x')")
	assert.Equal(t, "x", readValue(t, path))
}

func TestOpen_JournalOff(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)", "INSERT INTO t VALUES ('x')")
	assert.Equal(t, "x", readValue(t, path, JournalOff))
}

func TestOpen_ReadOnlyImmutable(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)", "INSERT INTO t VALUES ('x')")
	assert.Equal(t, "x", readValue(t, path, ReadOnly, Immutable))
}

func TestOpen_ReadOnlyRejectsWrites(t *testing.T) {
	path := createDB(t, "CREATE TABLE t (v TEXT)")
	database, err := Open(path, ReadOnly)
	require.NoError(t, err)
	defer database.Close()

	_, err = database.Exec("INSERT INTO t VALUES ('x')")
	require.Error(t, err)
}

func TestOpen_MissingFileCreatesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	_, err := Open(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database file")
	assert.NoFileExists(t, path)
}
