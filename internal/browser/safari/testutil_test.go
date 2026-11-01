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

type tabRow struct {
	uuid  string
	title string
}

// Every row gets subtype=2, the profile record the production query selects.
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

// Value is written as UTF-16 LE, as WebKit stores it.
type testLocalStorageItem struct {
	Key, Value string
}

// Every origin is written first-party, with top == frame.
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

// WebKit's SecurityOrigin layout: per block a scheme record, a host record, and a port marker; top, then frame.
func encodeOriginFile(topOrigin, frameOrigin string) []byte {
	var buf []byte
	buf = appendOriginBlock(buf, topOrigin)
	buf = appendOriginBlock(buf, frameOrigin)
	return buf
}

func appendOriginBlock(buf []byte, originURL string) []byte {
	scheme, host, port := splitTestOriginURL(originURL)
	buf = appendOriginRecord(buf, scheme)
	buf = appendOriginRecord(buf, host)
	if port == 0 {
		buf = append(buf, originPortDefaultMarker)
		return buf
	}
	buf = append(buf, originPortExplicitFlag)
	portBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(portBytes, port)
	return append(buf, portBytes...)
}

func appendOriginRecord(buf []byte, s string) []byte {
	lenBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBytes, uint32(len(s)))
	buf = append(buf, lenBytes...)
	buf = append(buf, originEncASCII)
	return append(buf, []byte(s)...)
}

// Port 0 means the URL had no explicit port.
func splitTestOriginURL(u string) (scheme, host string, port uint16) {
	idx := strings.Index(u, "://")
	if idx < 0 {
		return "", u, 0
	}
	scheme = u[:idx]
	rest := u[idx+3:]
	if colon := strings.LastIndex(rest, ":"); colon >= 0 {
		if p, err := strconv.ParseUint(rest[colon+1:], 10, 16); err == nil {
			return scheme, rest[:colon], uint16(p)
		}
	}
	return scheme, rest, 0
}

// addNullKey inserts a NULL-key row first, to exercise the reader's skip of NULL keys.
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

func encodeUTF16LE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, 2*len(u16))
	for i, r := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
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

func (c *testCollector) Add(r record.Record) { c.records = append(c.records, r) }
func (c *testCollector) Fail(err error)      { c.fails = append(c.fails, err) }

func collected[T any](t *testing.T, c *testCollector) []T {
	t.Helper()
	got := make([]T, 0, len(c.records))
	for _, r := range c.records {
		v, ok := any(r).(*T)
		require.Truef(t, ok, "collected a %T, want *%T", r, *new(T))
		got = append(got, *v)
	}
	return got
}
