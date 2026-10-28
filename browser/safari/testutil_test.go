package safari

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Real Safari table schemas — extracted via `sqlite3 History.db ".schema"`.
// ---------------------------------------------------------------------------

const safariHistoryItemsSchema = `CREATE TABLE history_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT NOT NULL UNIQUE,
	domain_expansion TEXT NULL,
	visit_count INTEGER NOT NULL DEFAULT 0,
	daily_visit_counts BLOB NOT NULL DEFAULT x'',
	weekly_visit_counts BLOB NULL,
	autocomplete_triggers BLOB NULL,
	should_recompute_derived_visit_counts INTEGER NOT NULL DEFAULT 1,
	visit_count_score INTEGER NOT NULL DEFAULT 0
)`

const safariHistoryVisitsSchema = `CREATE TABLE history_visits (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	history_item INTEGER NOT NULL REFERENCES history_items(id),
	visit_time REAL NOT NULL,
	title TEXT NULL,
	load_successful BOOLEAN NOT NULL DEFAULT 1,
	http_non_get INTEGER NOT NULL DEFAULT 0,
	synthesized INTEGER NOT NULL DEFAULT 0,
	redirect_source INTEGER NULL,
	redirect_destination INTEGER NULL,
	origin INTEGER NOT NULL DEFAULT 0,
	generation INTEGER NOT NULL DEFAULT 0,
	attributes INTEGER NOT NULL DEFAULT 0,
	score INTEGER NOT NULL DEFAULT 0
)`

// ---------------------------------------------------------------------------
// INSERT helpers
// ---------------------------------------------------------------------------

func insertHistoryItem(id int, url, domainExpansion string, visitCount int) string {
	return fmt.Sprintf(
		`INSERT INTO history_items (id, url, domain_expansion, visit_count)
		 VALUES (%d, '%s', '%s', %d)`,
		id, url, domainExpansion, visitCount,
	)
}

func insertHistoryVisit(id, historyItem int, visitTime float64, title string) string {
	return fmt.Sprintf(
		`INSERT INTO history_visits (id, history_item, visit_time, title)
		 VALUES (%d, %d, %f, '%s')`,
		id, historyItem, visitTime, title,
	)
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

func createTestDB(t *testing.T, name string, schemas []string, inserts ...string) string { //nolint:unparam // name will vary when future data types are added
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	for _, schema := range schemas {
		_, err = db.Exec(schema)
		require.NoError(t, err)
	}
	for _, stmt := range inserts {
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}
	return path
}

// ---------------------------------------------------------------------------
// SafariTabs.db fixtures
// ---------------------------------------------------------------------------

// tabRow describes one profile entry to stamp into the fake SafariTabs.db.
type tabRow struct {
	uuid  string
	title string
}

// writeSafariTabsDB creates a minimal SafariTabs.db at path containing only
// the bookmarks columns discoverSafariProfiles reads. Every row gets
// subtype=2 (profile record) so the production query picks it up.
func writeSafariTabsDB(t *testing.T, path string, rows []tabRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE bookmarks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		external_uuid TEXT,
		title TEXT,
		subtype INTEGER DEFAULT 0
	)`)
	require.NoError(t, err)

	for _, r := range rows {
		_, err = db.Exec(`INSERT INTO bookmarks (external_uuid, title, subtype) VALUES (?, ?, 2)`, r.uuid, r.title)
		require.NoError(t, err)
	}
}

// ---------------------------------------------------------------------------
// LocalStorage fixtures — modern WebKit nested Origins layout
// ---------------------------------------------------------------------------

// testLocalStorageItem is one key/value pair written to an ItemTable row.
// Value is encoded as UTF-16 LE, matching WebKit's on-disk format.
type testLocalStorageItem struct {
	Key, Value string
}

// buildTestLocalStorageDir creates a root dir that mirrors Safari 17+'s nested
// localStorage layout (<root>/<h1>/<h2>/origin + LocalStorage/localstorage.sqlite3)
// for each origin URL passed in. Origins are written as first-party (top == frame);
// for partitioned-origin coverage, use buildTestPartitionedLocalStorage.
func buildTestLocalStorageDir(t *testing.T, origins map[string][]testLocalStorageItem) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Origins")
	require.NoError(t, os.MkdirAll(root, 0o755))

	i := 0
	for origin, items := range origins {
		hash := fmt.Sprintf("h%02d", i)
		i++
		writeTestOriginStore(t, root, hash, hash, origin, origin, items)
	}
	return root
}

// writeTestOriginStore writes one <root>/<topHash>/<frameHash>/ tree with the given
// origins encoded into the binary origin file and items inserted into localstorage.sqlite3.
func writeTestOriginStore(t *testing.T, root, topHash, frameHash, topOrigin, frameOrigin string, items []testLocalStorageItem) {
	t.Helper()
	frameDir := filepath.Join(root, topHash, frameHash)
	require.NoError(t, os.MkdirAll(filepath.Join(frameDir, webkitLocalStorageSubdir), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(frameDir, webkitOriginFile), []byte(topOrigin+" "+frameOrigin), 0o644))

	dbPath := filepath.Join(frameDir, webkitLocalStorageSubdir, webkitLocalStorageDB)
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`)
	require.NoError(t, err)
	for _, item := range items {
		_, err = db.Exec(
			`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
			item.Key, encodeUTF16LE(item.Value),
		)
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())
}

// writeLocalStorageDB creates a minimal localstorage.sqlite3 at path with ItemTable populated
// from items. When addNullKey is true, a NULL-key row is inserted first to exercise the
// skip-NULL-key logic in readLocalStorageFile / countLocalStorageFile. This is a direct-DB
// variant of buildTestLocalStorageDir — use it when the test targets one DB, not the full
// Origins nesting.
func writeLocalStorageDB(t *testing.T, path string, items []testLocalStorageItem, addNullKey bool) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`)
	require.NoError(t, err)
	if addNullKey {
		_, err = db.Exec(
			`INSERT INTO ItemTable (key, value) VALUES (NULL, ?)`,
			encodeUTF16LE("null-key-sentinel"),
		)
		require.NoError(t, err)
	}
	for _, item := range items {
		_, err = db.Exec(
			`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
			item.Key, encodeUTF16LE(item.Value),
		)
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())
}

// encodeUTF16LE is the inverse of extract_storage.go's decodeUTF16LE — it mirrors
// the WebKit encoding so test fixtures round-trip through the extractor.
func encodeUTF16LE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, 2*len(u16))
	for i, r := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}
