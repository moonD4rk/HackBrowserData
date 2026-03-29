package chromium

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Shared test constants for Chromium encryption.
// Reusable across decrypt, cookie, password, and creditcard tests.
// ---------------------------------------------------------------------------

// testAESKey is a 16-byte AES-128 key for constructing test ciphertext.
var testAESKey = []byte("0123456789abcdef")

// ---------------------------------------------------------------------------
// Real Chrome table schemas — extracted via `sqlite3 <db> ".schema <table>"`.
// Using complete schemas ensures our SQL queries work against real browser data.
// ---------------------------------------------------------------------------

const loginsSchema = `CREATE TABLE logins (
	origin_url VARCHAR NOT NULL,
	action_url VARCHAR,
	username_element VARCHAR,
	username_value VARCHAR,
	password_element VARCHAR,
	password_value BLOB,
	submit_element VARCHAR,
	signon_realm VARCHAR NOT NULL,
	date_created INTEGER NOT NULL,
	blacklisted_by_user INTEGER NOT NULL,
	scheme INTEGER NOT NULL,
	password_type INTEGER,
	times_used INTEGER,
	form_data BLOB,
	display_name VARCHAR,
	icon_url VARCHAR,
	federation_url VARCHAR,
	skip_zero_click INTEGER,
	generation_upload_status INTEGER,
	possible_username_pairs BLOB,
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	date_last_used INTEGER NOT NULL DEFAULT 0,
	moving_blocked_for BLOB,
	date_password_modified INTEGER NOT NULL DEFAULT 0,
	sender_email VARCHAR,
	sender_name VARCHAR,
	date_received INTEGER,
	sharing_notification_displayed INTEGER NOT NULL DEFAULT 0,
	keychain_identifier BLOB,
	sender_profile_image_url VARCHAR,
	date_last_filled INTEGER NOT NULL DEFAULT 0,
	actor_login_approved INTEGER NOT NULL DEFAULT 0,
	UNIQUE (origin_url, username_element, username_value, password_element, signon_realm)
)`

const cookiesSchema = `CREATE TABLE cookies (
	creation_utc INTEGER NOT NULL,
	host_key TEXT NOT NULL,
	top_frame_site_key TEXT NOT NULL,
	name TEXT NOT NULL,
	value TEXT NOT NULL,
	encrypted_value BLOB NOT NULL,
	path TEXT NOT NULL,
	expires_utc INTEGER NOT NULL,
	is_secure INTEGER NOT NULL,
	is_httponly INTEGER NOT NULL,
	last_access_utc INTEGER NOT NULL,
	has_expires INTEGER NOT NULL,
	is_persistent INTEGER NOT NULL,
	priority INTEGER NOT NULL,
	samesite INTEGER NOT NULL,
	source_scheme INTEGER NOT NULL,
	source_port INTEGER NOT NULL,
	last_update_utc INTEGER NOT NULL,
	source_type INTEGER NOT NULL,
	has_cross_site_ancestor INTEGER NOT NULL
)`

const urlsSchema = `CREATE TABLE urls (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url LONGVARCHAR,
	title LONGVARCHAR,
	visit_count INTEGER DEFAULT 0 NOT NULL,
	typed_count INTEGER DEFAULT 0 NOT NULL,
	last_visit_time INTEGER NOT NULL,
	hidden INTEGER DEFAULT 0 NOT NULL
)`

const downloadsSchema = `CREATE TABLE downloads (
	id INTEGER PRIMARY KEY,
	guid VARCHAR NOT NULL,
	current_path LONGVARCHAR NOT NULL,
	target_path LONGVARCHAR NOT NULL,
	start_time INTEGER NOT NULL,
	received_bytes INTEGER NOT NULL,
	total_bytes INTEGER NOT NULL,
	state INTEGER NOT NULL,
	danger_type INTEGER NOT NULL,
	interrupt_reason INTEGER NOT NULL,
	hash BLOB NOT NULL,
	end_time INTEGER NOT NULL,
	opened INTEGER NOT NULL,
	last_access_time INTEGER NOT NULL,
	transient INTEGER NOT NULL,
	referrer VARCHAR NOT NULL,
	site_url VARCHAR NOT NULL,
	embedder_download_data VARCHAR NOT NULL,
	tab_url VARCHAR NOT NULL,
	tab_referrer_url VARCHAR NOT NULL,
	http_method VARCHAR NOT NULL,
	by_ext_id VARCHAR NOT NULL,
	by_ext_name VARCHAR NOT NULL,
	by_web_app_id VARCHAR NOT NULL,
	etag VARCHAR NOT NULL,
	last_modified VARCHAR NOT NULL,
	mime_type VARCHAR(255) NOT NULL,
	original_mime_type VARCHAR(255) NOT NULL
)`

const creditCardsSchema = `CREATE TABLE credit_cards (
	guid VARCHAR PRIMARY KEY,
	name_on_card VARCHAR,
	expiration_month INTEGER,
	expiration_year INTEGER,
	card_number_encrypted BLOB,
	date_modified INTEGER NOT NULL DEFAULT 0,
	origin VARCHAR DEFAULT '',
	use_count INTEGER NOT NULL DEFAULT 0,
	use_date INTEGER NOT NULL DEFAULT 0,
	billing_address_id VARCHAR,
	nickname VARCHAR
)`

// ---------------------------------------------------------------------------
// INSERT helpers — each returns one SQL statement with only the fields
// our extract functions care about; other NOT NULL columns get defaults.
// ---------------------------------------------------------------------------

func insertLogin(originURL, actionURL, username, pwdHex string, dateCreated int64) string {
	return fmt.Sprintf(
		`INSERT INTO logins (origin_url, action_url, username_element, username_value,
		 password_element, password_value, submit_element, signon_realm, date_created,
		 blacklisted_by_user, scheme)
		 VALUES ('%s', '%s', '', '%s', '', x'%s', '', '%s', %d, 0, 0)`,
		originURL, actionURL, username, pwdHex, originURL, dateCreated,
	)
}

func insertCookie(name, host, path, encValueHex string, creationUTC, expiresUTC int64, secure, httpOnly int) string {
	return fmt.Sprintf(
		`INSERT INTO cookies (creation_utc, host_key, top_frame_site_key, name, value,
		 encrypted_value, path, expires_utc, is_secure, is_httponly, last_access_utc,
		 has_expires, is_persistent, priority, samesite, source_scheme, source_port,
		 last_update_utc, source_type, has_cross_site_ancestor)
		 VALUES (%d, '%s', '', '%s', '', x'%s', '%s', %d, %d, %d, %d, 1, 1, 1, 0, 2, 443, %d, 0, 0)`,
		creationUTC, host, name, encValueHex, path, expiresUTC, secure, httpOnly, creationUTC, creationUTC,
	)
}

func insertURL(url, title string, visitCount int, lastVisitTime int64) string {
	return fmt.Sprintf(
		`INSERT INTO urls (url, title, visit_count, typed_count, last_visit_time, hidden)
		 VALUES ('%s', '%s', %d, 0, %d, 0)`,
		url, title, visitCount, lastVisitTime,
	)
}

func insertDownload(targetPath, tabURL string, totalBytes, startTime, endTime int64) string {
	return fmt.Sprintf(
		`INSERT INTO downloads (id, guid, current_path, target_path, start_time, received_bytes,
		 total_bytes, state, danger_type, interrupt_reason, hash, end_time, opened, last_access_time,
		 transient, referrer, site_url, embedder_download_data, tab_url, tab_referrer_url,
		 http_method, by_ext_id, by_ext_name, by_web_app_id, etag, last_modified, mime_type, original_mime_type)
		 VALUES (NULL, '', '', '%s', %d, %d, %d, 1, 0, 0, x'', %d, 0, 0, 0, '', '', '', '%s', '', 'GET', '', '', '', '', '', '', '')`,
		targetPath, startTime, totalBytes, totalBytes, endTime, tabURL,
	)
}

func insertCreditCard(name string, month, year int, encNumberHex string) string {
	return fmt.Sprintf(
		`INSERT INTO credit_cards (guid, name_on_card, expiration_month, expiration_year, card_number_encrypted)
		 VALUES ('%s-%d-%d', '%s', %d, %d, x'%s')`,
		name, month, year, name, month, year, encNumberHex,
	)
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

// createTestDB creates a SQLite database with the given schema and insert statements.
func createTestDB(t *testing.T, name, schema string, inserts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(schema)
	require.NoError(t, err)

	for _, stmt := range inserts {
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}
	return path
}

// createTestJSON creates a file with the given JSON content.
func createTestJSON(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// createTestLevelDB creates a LevelDB directory with the given key-value pairs.
func createTestLevelDB(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "leveldb")
	db, err := leveldb.OpenFile(dir, nil)
	require.NoError(t, err)
	for k, v := range entries {
		require.NoError(t, db.Put([]byte(k), []byte(v), nil))
	}
	require.NoError(t, db.Close())
	return dir
}
