package firefox

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Real Firefox table schemas — extracted via `sqlite3 <db> ".schema <table>"`.
// ---------------------------------------------------------------------------

const mozCookiesSchema = `CREATE TABLE moz_cookies (
	id INTEGER PRIMARY KEY,
	originAttributes TEXT NOT NULL DEFAULT '',
	name TEXT,
	value TEXT,
	host TEXT,
	path TEXT,
	expiry INTEGER,
	lastAccessed INTEGER,
	creationTime INTEGER,
	isSecure INTEGER,
	isHttpOnly INTEGER,
	inBrowserElement INTEGER DEFAULT 0,
	sameSite INTEGER DEFAULT 0,
	rawSameSite INTEGER DEFAULT 0,
	schemeMap INTEGER DEFAULT 0,
	isPartitionedAttributeSet INTEGER DEFAULT 0,
	CONSTRAINT moz_uniqueid UNIQUE (name, host, path, originAttributes)
)`

const mozPlacesSchema = `CREATE TABLE moz_places (
	id INTEGER PRIMARY KEY,
	url LONGVARCHAR,
	title LONGVARCHAR,
	rev_host LONGVARCHAR,
	visit_count INTEGER DEFAULT 0,
	hidden INTEGER DEFAULT 0 NOT NULL,
	typed INTEGER DEFAULT 0 NOT NULL,
	frecency INTEGER DEFAULT -1 NOT NULL,
	last_visit_date INTEGER,
	guid TEXT,
	foreign_count INTEGER DEFAULT 0 NOT NULL,
	url_hash INTEGER DEFAULT 0 NOT NULL,
	description TEXT,
	preview_image_url TEXT,
	site_name TEXT,
	origin_id INTEGER,
	recalc_frecency INTEGER NOT NULL DEFAULT 0,
	alt_frecency INTEGER,
	recalc_alt_frecency INTEGER NOT NULL DEFAULT 0
)`

const mozBookmarksSchema = `CREATE TABLE moz_bookmarks (
	id INTEGER PRIMARY KEY,
	type INTEGER,
	fk INTEGER DEFAULT NULL,
	parent INTEGER,
	position INTEGER,
	title LONGVARCHAR,
	keyword_id INTEGER,
	folder_type TEXT,
	dateAdded INTEGER,
	lastModified INTEGER,
	guid TEXT,
	syncStatus INTEGER NOT NULL DEFAULT 0,
	syncChangeCounter INTEGER NOT NULL DEFAULT 1
)`

const mozAnnosSchema = `CREATE TABLE moz_annos (
	id INTEGER PRIMARY KEY,
	place_id INTEGER NOT NULL,
	anno_attribute_id INTEGER,
	content LONGVARCHAR,
	flags INTEGER DEFAULT 0,
	expiration INTEGER DEFAULT 0,
	type INTEGER DEFAULT 0,
	dateAdded INTEGER DEFAULT 0,
	lastModified INTEGER DEFAULT 0
)`

const webappsstore2Schema = `CREATE TABLE webappsstore2 (
	originAttributes TEXT,
	originKey TEXT,
	scope TEXT,
	key TEXT,
	value TEXT
)`

// ---------------------------------------------------------------------------
// INSERT helpers
// ---------------------------------------------------------------------------

func insertMozCookie(name, value, host, path string, creationTime, expiry int64, isSecure, isHTTPOnly int) string {
	return fmt.Sprintf(
		`INSERT INTO moz_cookies (name, value, host, path, creationTime, expiry, isSecure, isHttpOnly, lastAccessed)
		 VALUES ('%s', '%s', '%s', '%s', %d, %d, %d, %d, %d)`,
		name, value, host, path, creationTime, expiry, isSecure, isHTTPOnly, creationTime,
	)
}

func insertMozPlace(id int, url, title string, visitCount int, lastVisitDate int64) string {
	return fmt.Sprintf(
		`INSERT INTO moz_places (id, url, title, visit_count, last_visit_date, rev_host, guid, url_hash)
		 VALUES (%d, '%s', '%s', %d, %d, '', 'guid-%d', 0)`,
		id, url, title, visitCount, lastVisitDate, id,
	)
}

func insertMozBookmark(id, fk, bookmarkType int, title string, dateAdded int64) string {
	return fmt.Sprintf(
		`INSERT INTO moz_bookmarks (id, type, fk, parent, position, title, dateAdded, lastModified, guid)
		 VALUES (%d, %d, %d, 0, 0, '%s', %d, %d, 'bm-guid-%d')`,
		id, bookmarkType, fk, title, dateAdded, dateAdded, id,
	)
}

func insertMozAnno(placeID int, content string, dateAdded int64) string {
	return fmt.Sprintf(
		`INSERT INTO moz_annos (place_id, anno_attribute_id, content, dateAdded, lastModified)
		 VALUES (%d, 1, '%s', %d, %d)`,
		placeID, content, dateAdded, dateAdded,
	)
}

func insertWebappsstore(originKey, key, value string) string {
	return fmt.Sprintf(
		`INSERT INTO webappsstore2 (originAttributes, originKey, scope, key, value)
		 VALUES ('', '%s', '', '%s', '%s')`,
		originKey, key, value,
	)
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

// installFile copies a test fixture file into a profile directory.
func installFile(t *testing.T, profileDir, srcPath, dstName string) {
	t.Helper()
	data, err := os.ReadFile(srcPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, dstName), data, 0o644))
}

func createTestDB(t *testing.T, name string, schemas []string, inserts ...string) string {
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

func createTestJSON(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func setupMozBookmarkDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozBookmarksSchema},
		insertMozPlace(1, "https://go.dev", "Go", 0, 0),
		insertMozPlace(2, "https://github.com", "GitHub", 0, 0),
		insertMozBookmark(1, 1, 1, "Go Website", 1700000000000000),
		insertMozBookmark(2, 2, 1, "GitHub", 1710000000000000),
	)
}

func setupMozCookieDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "cookies.sqlite", []string{mozCookiesSchema},
		insertMozCookie("session", "abc123", ".example.com", "/", 1700000000000000, 1800000000, 1, 1),
		insertMozCookie("token", "xyz789", ".new.com", "/api", 1710000000000000, 1810000000, 1, 0),
	)
}

func setupMozDownloadDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozAnnosSchema},
		insertMozPlace(1, "https://example.com/old.zip", "Old File", 0, 0),
		insertMozPlace(2, "https://example.com/new.pdf", "New File", 0, 0),
		insertMozAnno(1, "/tmp/old.zip", 1700000000000000),
		insertMozAnno(2, "/tmp/new.pdf", 1710000000000000),
	)
}

func setupMozExtensionJSON(t *testing.T) string {
	t.Helper()
	return createTestJSON(t, "extensions.json", `{
		"addons": [
			{
				"id": "ublock@gorhill.org",
				"location": "app-profile",
				"version": "1.52.0",
				"active": true,
				"defaultLocale": {
					"name": "uBlock Origin",
					"description": "An efficient blocker"
				}
			},
			{
				"id": "system@mozilla.org",
				"location": "app-system-defaults",
				"version": "1.0",
				"defaultLocale": {"name": "System Addon"}
			},
			{
				"id": "bitwarden@bitwarden.com",
				"location": "app-profile",
				"version": "2024.1.0",
				"active": true,
				"defaultLocale": {
					"name": "Bitwarden",
					"description": "Password manager"
				}
			}
		]
	}`)
}

func setupMozHistoryDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "places.sqlite", []string{mozPlacesSchema},
		insertMozPlace(1, "https://github.com", "GitHub", 100, 1700000000000000),
		insertMozPlace(2, "https://go.dev", "Go", 50, 1710000000000000),
		insertMozPlace(3, "https://example.com", "Example", 200, 1690000000000000),
	)
}

func setupWebappsDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "webappsstore.sqlite", []string{webappsstore2Schema},
		insertWebappsstore("moc.buhtig.:https:443", "theme", "dark"),
		insertWebappsstore("moc.buhtig.:https:443", "lang", "en"),
		insertWebappsstore("moc.elpmaxe.:http:8080", "token", "abc123"),
	)
}
