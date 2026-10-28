package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Only GCM-sealed values (v10 under a 32-byte key, v20) decrypt identically on every OS.
var chromiumFixtureKeys = masterkey.Set{
	masterkey.V10: []byte("0123456789abcdef0123456789abcdef"),
	masterkey.V20: []byte("0123456789abcdef"),
}

var chromiumFixtureInstalls = []struct {
	key  string
	name string
}{
	{key: "chrome", name: "Chrome"},
	{key: "edge", name: "Microsoft Edge"},
}

var chromiumFixtureProfiles = []struct {
	dir           string
	label         string
	start         time.Time
	accountLogins bool
}{
	{dir: "Default", label: "home", start: time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC), accountLogins: true},
	{dir: "Profile 1", label: "work", start: time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)},
}

func chromiumFixtureInstallations(t *testing.T, root string) {
	t.Helper()
	for _, inst := range chromiumFixtureInstalls {
		chromiumFixtureUserData(t, filepath.Join(root, inst.key), inst.key)
	}
}

func chromiumFixtureBrowsers(t *testing.T, root string) []browser.Browser {
	t.Helper()
	browsers := make([]browser.Browser, 0, len(chromiumFixtureInstalls))
	for _, inst := range chromiumFixtureInstalls {
		b, err := chromium.NewBrowser(types.BrowserConfig{
			Key:         inst.key,
			Name:        inst.name,
			Kind:        types.Chromium,
			UserDataDir: filepath.Join(root, inst.key),
		})
		require.NoError(t, err)
		require.NotNil(t, b)
		b.SetKeys(chromiumFixtureKeys)
		browsers = append(browsers, b)
	}
	return browsers
}

func chromiumFixtureUserData(t *testing.T, userDataDir, browserKey string) {
	t.Helper()
	fixtureWrite(t, filepath.Join(userDataDir, "Local State"), "{}")
	for _, p := range chromiumFixtureProfiles {
		site := p.label + "." + browserKey + ".test"
		chromiumFixtureProfile(t, filepath.Join(userDataDir, p.dir), site, p.start, p.accountLogins)
	}
}

func chromiumFixtureProfile(t *testing.T, dir, site string, start time.Time, accountLogins bool) {
	t.Helper()
	at := func(offset time.Duration) int64 { return chromiumFixtureMicros(start.Add(offset)) }
	link := func(host, path string) string { return "https://" + host + "." + site + path }
	v10 := func(plaintext string) string {
		return chromiumFixtureSeal(t, "v10", chromiumFixtureKeys[masterkey.V10], plaintext)
	}
	v20 := func(plaintext string) string {
		return chromiumFixtureSeal(t, "v20", chromiumFixtureKeys[masterkey.V20], plaintext)
	}
	// Chrome 130+ seals SHA256(host_key) ahead of the cookie value, which extraction must strip again.
	hostBound := func(host, value string) string {
		sum := sha256.Sum256([]byte(host))
		return string(sum[:]) + value
	}
	day := 24 * time.Hour

	fixtureWrite(t, filepath.Join(dir, "Preferences"), "{}")

	fixtureSQLite(t, filepath.Join(dir, "Login Data"), chromiumFixtureLoginsSchema,
		chromiumFixtureInsertLogin(link("accounts", "/"), link("accounts", "/login"), "alice",
			v10("correct-horse-battery-staple"), at(time.Hour)),
		chromiumFixtureInsertLogin(link("shop", "/"), link("shop", "/signin"), "alice@"+site, v10(`pa,ss"word`), at(2*time.Hour)),
	)
	if accountLogins {
		fixtureSQLite(t, filepath.Join(dir, "Login Data For Account"), chromiumFixtureLoginsSchema,
			chromiumFixtureInsertLogin(link("sync", "/"), link("sync", "/auth"), "alice", v10("synced-secret"), at(90*time.Minute)),
		)
	}

	domain, www, app := "."+site, "www."+site, "app."+site
	fixtureSQLite(t, filepath.Join(dir, "Network", "Cookies"), chromiumFixtureCookiesSchema,
		chromiumFixtureInsertCookie("SID", domain, "/", v20(hostBound(domain, "sid-"+site)),
			at(10*time.Minute+123456*time.Microsecond), at(365*day), 1, 1, 1),
		chromiumFixtureInsertCookie("prefs", www, "/settings", v10("theme=dark"), at(20*time.Minute), at(30*day), 0, 0, 2),
		chromiumFixtureInsertCookie("tracker", domain, "/", v20(hostBound(domain, "opt-out")), at(30*time.Minute), at(730*day), 1, 0, 0),
		chromiumFixtureInsertCookie("session", app, "/", v20("ephemeral"), at(40*time.Minute), 0, 1, 1, -1),
	)

	fixtureSQLite(t, filepath.Join(dir, "History"), chromiumFixtureURLsSchema, chromiumFixtureDownloadsSchema,
		chromiumFixtureInsertURL(link("www", "/"), "Home", 42, at(3*time.Hour)),
		chromiumFixtureInsertURL(link("docs", "/guide?q=go&lang=en"), `Guide, "quoted" & <b>bold</b>`, 7, at(4*time.Hour)),
		chromiumFixtureInsertURL(link("news", "/ja"), "ニュース", 3, at(5*time.Hour)),
		chromiumFixtureInsertDownload(`C:\Users\alice\Downloads\report.pdf`, link("files", "/report.pdf"), "application/pdf",
			52344, at(6*time.Hour), at(6*time.Hour+3*time.Second)),
		chromiumFixtureInsertDownload("/home/alice/Downloads/archive.tar.gz", link("files", "/archive.tar.gz"), "application/gzip",
			1048576, at(7*time.Hour), 0),
	)

	fixtureWrite(t, filepath.Join(dir, "Bookmarks"),
		fmt.Sprintf(chromiumFixtureBookmarks, site, at(0), at(8*time.Hour), at(9*time.Hour), at(10*time.Hour)))

	fixtureSQLite(t, filepath.Join(dir, "Web Data"), chromiumFixtureCreditCardsSchema,
		chromiumFixtureInsertCreditCard("Alice Example", 4, 2031, v10("4111111111111111"), "Travel", "billing-home"),
		chromiumFixtureInsertCreditCard("Bob Example", 11, 2029, v10("5500005555555559"), "", ""),
	)

	fixtureWrite(t, filepath.Join(dir, "Secure Preferences"), chromiumFixtureSecurePreferences)

	origin := link("app", "")
	dataKey := "_" + origin + "\x00"
	chromiumFixtureLevelDB(t, filepath.Join(dir, "Local Storage", "leveldb"), map[string]string{
		"VERSION":              "1",
		"META:" + origin:       "\x08\x96\x01",
		"METAACCESS:" + origin: "\x10\x20",
		dataKey + chromiumFixtureLatin1("greeting"): chromiumFixtureLatin1("caf\xe9"),
		dataKey + chromiumFixtureUTF16("言語"):        chromiumFixtureUTF16("日本語"),
		dataKey + chromiumFixtureLatin1("blob"):     chromiumFixtureLatin1(strings.Repeat("x", 2100)),
	})

	namespace := "namespace-0a1b2c3d_4e5f_6a7b_8c9d_0e1f2a3b4c5d-"
	chromiumFixtureLevelDB(t, filepath.Join(dir, "Session Storage"), map[string]string{
		namespace + link("app", "/"):  "1",
		namespace + link("shop", "/"): "2",
		"map-1-draft":                 chromiumFixtureUTF16Raw("Hello, world"),
		"map-1-scrollY":               chromiumFixtureUTF16Raw("640"),
		"map-2-cart":                  chromiumFixtureUTF16Raw(`{"items":2}`),
		"next-map-id":                 "3",
		"version":                     "1",
	})
}

func chromiumFixtureSeal(t *testing.T, version string, key []byte, plaintext string) string {
	t.Helper()
	nonce := []byte(version + "_nonce_12")
	sealed, err := crypto.AESGCMEncrypt(key, nonce, []byte(plaintext))
	require.NoError(t, err)
	return hex.EncodeToString(append(append([]byte(version), nonce...), sealed...))
}

func chromiumFixtureMicros(ts time.Time) int64 {
	const unixToChromiumEpochMicros = 11_644_473_600_000_000
	return ts.UnixMicro() + unixToChromiumEpochMicros
}

func chromiumFixtureLatin1(s string) string { return "\x01" + s }

func chromiumFixtureUTF16(s string) string { return "\x00" + chromiumFixtureUTF16Raw(s) }

func chromiumFixtureUTF16Raw(s string) string {
	units := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[2*i:], u)
	}
	return string(b)
}

func chromiumFixtureLevelDB(t *testing.T, dir string, entries map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	db, err := leveldb.OpenFile(dir, nil)
	require.NoError(t, err)
	for _, k := range keys {
		require.NoError(t, db.Put([]byte(k), []byte(entries[k]), nil))
	}
	require.NoError(t, db.Close())
}

func chromiumFixtureInsertLogin(originURL, actionURL, username, pwdHex string, dateCreated int64) string {
	return fmt.Sprintf(
		`INSERT INTO logins (origin_url, action_url, username_element, username_value,
		 password_element, password_value, submit_element, signon_realm, date_created,
		 blacklisted_by_user, scheme)
		 VALUES ('%s', '%s', '', '%s', '', x'%s', '', '%s', %d, 0, 0)`,
		originURL, actionURL, username, pwdHex, originURL, dateCreated,
	)
}

func chromiumFixtureInsertCookie(
	name, host, path, encValueHex string, creationUTC, expiresUTC int64, secure, httpOnly, sameSite int,
) string {
	persistent := 0
	if expiresUTC != 0 {
		persistent = 1
	}
	return fmt.Sprintf(
		`INSERT INTO cookies (creation_utc, host_key, top_frame_site_key, name, value,
		 encrypted_value, path, expires_utc, is_secure, is_httponly, last_access_utc,
		 has_expires, is_persistent, priority, samesite, source_scheme, source_port,
		 last_update_utc, source_type, has_cross_site_ancestor)
		 VALUES (%d, '%s', '', '%s', '', x'%s', '%s', %d, %d, %d, %d, %d, %d, 1, %d, 2, 443, %d, 0, 0)`,
		creationUTC, host, name, encValueHex, path, expiresUTC, secure, httpOnly, creationUTC,
		persistent, persistent, sameSite, creationUTC,
	)
}

func chromiumFixtureInsertURL(url, title string, visitCount int, lastVisitTime int64) string {
	return fmt.Sprintf(
		`INSERT INTO urls (url, title, visit_count, typed_count, last_visit_time, hidden)
		 VALUES ('%s', '%s', %d, 0, %d, 0)`,
		url, title, visitCount, lastVisitTime,
	)
}

func chromiumFixtureInsertDownload(targetPath, tabURL, mimeType string, totalBytes, startTime, endTime int64) string {
	return fmt.Sprintf(
		`INSERT INTO downloads (id, guid, current_path, target_path, start_time, received_bytes,
		 total_bytes, state, danger_type, interrupt_reason, hash, end_time, opened, last_access_time,
		 transient, referrer, site_url, embedder_download_data, tab_url, tab_referrer_url,
		 http_method, by_ext_id, by_ext_name, by_web_app_id, etag, last_modified, mime_type, original_mime_type)
		 VALUES (NULL, '', '', '%s', %d, %d, %d, 1, 0, 0, x'', %d, 0, 0, 0, '', '', '', '%s', '', 'GET', '', '', '', '', '', '%s', '')`,
		targetPath, startTime, totalBytes, totalBytes, endTime, tabURL, mimeType,
	)
}

func chromiumFixtureInsertCreditCard(name string, month, year int, encNumberHex, nickName, address string) string {
	return fmt.Sprintf(
		`INSERT INTO credit_cards (guid, name_on_card, expiration_month, expiration_year, card_number_encrypted,
		 nickname, billing_address_id)
		 VALUES ('%s-%d-%d', '%s', %d, %d, x'%s', '%s', '%s')`,
		name, month, year, name, month, year, encNumberHex, nickName, address,
	)
}

const chromiumFixtureLoginsSchema = `CREATE TABLE logins (
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

const chromiumFixtureCookiesSchema = `CREATE TABLE cookies (
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

const chromiumFixtureURLsSchema = `CREATE TABLE urls (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url LONGVARCHAR,
	title LONGVARCHAR,
	visit_count INTEGER DEFAULT 0 NOT NULL,
	typed_count INTEGER DEFAULT 0 NOT NULL,
	last_visit_time INTEGER NOT NULL,
	hidden INTEGER DEFAULT 0 NOT NULL
)`

const chromiumFixtureDownloadsSchema = `CREATE TABLE downloads (
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

const chromiumFixtureCreditCardsSchema = `CREATE TABLE credit_cards (
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

const chromiumFixtureBookmarks = `{
  "checksum": "00000000000000000000000000000000",
  "roots": {
    "bookmark_bar": {
      "children": [
        {"date_added": "%[3]d", "id": "4", "name": "Docs", "type": "url", "url": "https://docs.%[1]s/"},
        {
          "children": [
            {"date_added": "%[4]d", "id": "6", "name": "Long read <with> & tags", "type": "url", "url": "https://news.%[1]s/long-read"}
          ],
          "date_added": "%[2]d",
          "id": "5",
          "name": "Reading",
          "type": "folder"
        }
      ],
      "date_added": "%[2]d",
      "id": "1",
      "name": "Bookmarks bar",
      "type": "folder"
    },
    "other": {
      "children": [
        {"date_added": "%[5]d", "id": "7", "name": "Café menu", "type": "url", "url": "https://shop.%[1]s/menu"}
      ],
      "date_added": "%[2]d",
      "id": "2",
      "name": "Other bookmarks",
      "type": "folder"
    },
    "synced": {"children": [], "date_added": "%[2]d", "id": "3", "name": "Mobile bookmarks", "type": "folder"}
  },
  "version": 1
}`

const chromiumFixtureSecurePreferences = `{
  "extensions": {
    "settings": {
      "aaaabbbbccccddddeeeeffffgggghhhh": {
        "disable_reasons": [],
        "location": 1,
        "manifest": {
          "description": "Groups tabs by site, \"neatly\" & <fast>",
          "homepage_url": "https://tabs.example/",
          "name": "Tab Organizer",
          "version": "2.4.1"
        }
      },
      "iiiijjjjkkkkllllmmmmnnnnoooopppp": {
        "disable_reasons": [1],
        "location": 1,
        "manifest": {"description": "Distraction-free reading", "name": "Reader Mode", "version": "0.9.0"}
      },
      "ppppoooonnnnmmmmllllkkkkjjjjiiii": {
        "location": 5,
        "manifest": {"name": "Built-in Component", "version": "1.0"}
      },
      "hhhhggggffffeeeeddddccccbbbbaaaa": {
        "location": 10,
        "manifest": {"name": "External Component", "version": "1.0"}
      },
      "abcdabcdabcdabcdabcdabcdabcdabcd": {
        "location": 4,
        "manifest": {"name": "Legacy Unpacked", "version": "0.1"},
        "state": 1
      },
      "dcbadcbadcbadcbadcbadcbadcbadcba": {
        "location": 1
      }
    }
  }
}`
