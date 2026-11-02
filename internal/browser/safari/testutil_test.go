package safari

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// Real Safari schemas from sqlite3 History.db .schema, so the queries are tested against the columns Safari has.

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

func createTestDB(t *testing.T, name string, schemas []string, inserts ...string) string { //nolint:unparam // name will vary when future data types are added
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer database.Close()

	for _, schema := range schemas {
		_, err = database.Exec(schema)
		require.NoError(t, err)
	}
	for _, statement := range inserts {
		_, err = database.Exec(statement)
		require.NoError(t, err)
	}
	return path
}

type tabRow struct {
	uuid  string
	title string
}

// writeSafariTabsDB gives every row subtype=2, the profile record the production query selects.
func writeSafariTabsDB(t *testing.T, path string, rows []tabRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer database.Close()

	_, err = database.Exec(`CREATE TABLE bookmarks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		external_uuid TEXT,
		title TEXT,
		subtype INTEGER DEFAULT 0
	)`)
	require.NoError(t, err)

	for _, row := range rows {
		_, err = database.Exec(`INSERT INTO bookmarks (external_uuid, title, subtype) VALUES (?, ?, 2)`, row.uuid, row.title)
		require.NoError(t, err)
	}
}

// testLocalStorageItem holds a Value written as UTF-16 LE, as WebKit stores it.
type testLocalStorageItem struct {
	Key, Value string
}

// buildTestLocalStorageDir writes every origin first-party, with top == frame.
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

func writeTestOriginStore(t *testing.T, root, topHash, frameHash, topOrigin, frameOrigin string, items []testLocalStorageItem) {
	t.Helper()
	frameDir := filepath.Join(root, topHash, frameHash)
	require.NoError(t, os.MkdirAll(filepath.Join(frameDir, webkitLocalStorageSubdir), 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(frameDir, webkitOriginFile),
		encodeOriginFile(topOrigin, frameOrigin),
		0o644,
	))

	dbPath := filepath.Join(frameDir, webkitLocalStorageSubdir, webkitLocalStorageDB)
	database, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = database.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`)
	require.NoError(t, err)
	for _, item := range items {
		_, err = database.Exec(
			`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
			item.Key, encodeUTF16LE(item.Value),
		)
		require.NoError(t, err)
	}
	require.NoError(t, database.Close())
}

// encodeOriginFile follows WebKit's SecurityOrigin layout: per block a scheme, a host, and a port; top, then frame.
func encodeOriginFile(topOrigin, frameOrigin string) []byte {
	var encoded []byte
	encoded = appendOriginBlock(encoded, topOrigin)
	encoded = appendOriginBlock(encoded, frameOrigin)
	return encoded
}

func appendOriginBlock(encoded []byte, originURL string) []byte {
	scheme, host, port := splitTestOriginURL(originURL)
	encoded = appendOriginRecord(encoded, scheme)
	encoded = appendOriginRecord(encoded, host)
	if port == 0 {
		encoded = append(encoded, originPortDefaultMarker)
		return encoded
	}
	encoded = append(encoded, originPortExplicitFlag)
	portBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(portBytes, port)
	return append(encoded, portBytes...)
}

func appendOriginRecord(encoded []byte, text string) []byte {
	lenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBytes, uint32(len(text)))
	encoded = append(encoded, lenBytes...)
	encoded = append(encoded, originEncASCII)
	return append(encoded, []byte(text)...)
}

// splitTestOriginURL returns port 0 when the URL had no explicit port.
func splitTestOriginURL(originURL string) (scheme, host string, port uint16) {
	index := strings.Index(originURL, "://")
	if index < 0 {
		return "", originURL, 0
	}
	scheme = originURL[:index]
	rest := originURL[index+3:]
	if colon := strings.LastIndex(rest, ":"); colon >= 0 {
		if parsedPort, err := strconv.ParseUint(rest[colon+1:], 10, 16); err == nil {
			return scheme, rest[:colon], uint16(parsedPort)
		}
	}
	return scheme, rest, 0
}

// writeLocalStorageDB inserts a NULL-key row first when addNullKey is set, to exercise the reader's skip of NULL keys.
func writeLocalStorageDB(t *testing.T, path string, items []testLocalStorageItem, addNullKey bool) {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = database.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`)
	require.NoError(t, err)
	if addNullKey {
		_, err = database.Exec(
			`INSERT INTO ItemTable (key, value) VALUES (NULL, ?)`,
			encodeUTF16LE("null-key-sentinel"),
		)
		require.NoError(t, err)
	}
	for _, item := range items {
		_, err = database.Exec(
			`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
			item.Key, encodeUTF16LE(item.Value),
		)
		require.NoError(t, err)
	}
	require.NoError(t, database.Close())
}

func encodeUTF16LE(text string) []byte {
	units := utf16.Encode([]rune(text))
	encoded := make([]byte, 2*len(units))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(encoded[i*2:], unit)
	}
	return encoded
}

func mkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
}

type testCollector struct {
	records []record.Record
	fails   []error
}

func (c *testCollector) Add(row record.Record) { c.records = append(c.records, row) }
func (c *testCollector) Fail(err error)        { c.fails = append(c.fails, err) }

func collected[T any](t *testing.T, collector *testCollector) []T {
	t.Helper()
	got := make([]T, 0, len(collector.records))
	for _, row := range collector.records {
		value, ok := any(row).(*T)
		require.Truef(t, ok, "collected a %T, want *%T", row, *new(T))
		got = append(got, *value)
	}
	return got
}
