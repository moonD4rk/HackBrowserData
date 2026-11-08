package browsertest

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func writeSQLite(t *testing.T, path string, statements ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer database.Close()
	for _, statement := range statements {
		_, err = database.Exec(statement)
		require.NoError(t, err)
	}
}
