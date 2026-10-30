package browsertest

import (
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
)

// home's logins open only with its second key candidate (AES-256) and work's with its only one (3DES), so the golden
// covers the candidate check and both login ciphers.
var firefoxFixtureProfiles = []struct {
	dir        string
	label      string
	start      time.Time
	globalSalt []byte
	keys       [][]byte
}{
	{
		dir: "abcd1234.default-release", label: "home", start: time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC),
		globalSalt: []byte("home-global-salt-20b"),
		keys:       [][]byte{[]byte("home-3des-key-0123456789"), []byte("home-aes256-key-0123456789abcdef")},
	},
	{
		dir: "wxyz5678.work", label: "work", start: time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC),
		globalSalt: []byte("work-global-salt-20b"),
		keys:       [][]byte{[]byte("work-3des-key-0123456789")},
	},
}

var firefoxFixtureKeyID = []byte{0xf8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

var (
	oidPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidPBKDF2         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidHMACWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidAES256CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
	oidDESEDE3CBC     = asn1.ObjectIdentifier{1, 2, 840, 113549, 3, 7}
)

func FirefoxUserData(t *testing.T, profilesDir string) {
	t.Helper()
	fixtureWrite(t, filepath.Join(profilesDir, "profiles.ini"), "[General]\nStartWithLastProfile=1\n")
	fixtureWrite(t, filepath.Join(profilesDir, "Crash Reports", "InstallTime20240101000000"), "1704067200")
	for _, p := range firefoxFixtureProfiles {
		firefoxFixtureProfile(t, filepath.Join(profilesDir, p.dir), p.label+".firefox.test", p.start, p.globalSalt, p.keys)
	}
}

func firefoxFixtureProfile(t *testing.T, dir, site string, start time.Time, globalSalt []byte, keys [][]byte) {
	t.Helper()
	micros := func(offset time.Duration) int64 { return start.Add(offset).UnixMicro() }
	millis := func(offset time.Duration) int64 { return start.Add(offset).UnixMilli() }
	seconds := func(offset time.Duration) int64 { return start.Add(offset).Unix() }
	link := func(host, path string) string { return "https://" + host + "." + site + path }
	day := 24 * time.Hour

	key4 := []string{
		firefoxFixtureMetaDataSchema,
		firefoxFixtureNSSPrivateSchema,
		fmt.Sprintf(`INSERT INTO metaData (id, item1, item2) VALUES ('password', x'%x', x'%x')`,
			globalSalt, firefoxFixturePBES2Seal(t, globalSalt, site+"/password-check", []byte("password-check"))),
	}
	for i, key := range keys {
		key4 = append(key4, fmt.Sprintf(`INSERT INTO nssPrivate (id, a11, a102) VALUES (%d, x'%x', x'%x')`,
			i+1, firefoxFixturePBES2Seal(t, globalSalt, fmt.Sprintf("%s/key-%d", site, i), key), firefoxFixtureKeyID))
	}
	fixtureSQLite(t, filepath.Join(dir, "key4.db"), key4...)

	loginKey := keys[len(keys)-1]
	seal := func(field, plaintext string) string {
		return firefoxFixtureLoginSeal(t, loginKey, site+"/"+field, plaintext)
	}
	fixtureWrite(t, filepath.Join(dir, "logins.json"), fmt.Sprintf(firefoxFixtureLogins, site,
		seal("1/username", "alice"), seal("1/password", "correct-horse-battery-staple"), millis(time.Hour+250*time.Millisecond),
		seal("2/username", "alice@"+site), seal("2/password", `pa,ss"word`), millis(2*time.Hour),
		seal("3/username", "alice"), seal("3/password", "pässwörd"), millis(90*time.Minute),
	))

	domain := "." + site
	fixtureSQLite(t, filepath.Join(dir, "cookies.sqlite"), firefoxFixtureCookiesSchema,
		firefoxFixtureInsertCookie("SID", "sid-"+site, domain, "/", seconds(365*day), micros(10*time.Minute+123456*time.Microsecond), 1, 1, 1),
		firefoxFixtureInsertCookie("prefs", "theme=dark", "www."+site, "/settings", seconds(30*day), micros(20*time.Minute), 0, 0, 2),
		firefoxFixtureInsertCookie("tracker", "opt-out", domain, "/", seconds(730*day), micros(30*time.Minute), 1, 0, 0),
		firefoxFixtureInsertCookie("session", "ephemeral", "app."+site, "/", 0, micros(40*time.Minute), 1, 1, 256),
	)

	fixtureSQLite(t, filepath.Join(dir, "places.sqlite"),
		firefoxFixturePlacesSchema, firefoxFixtureBookmarksSchema, firefoxFixtureAnnoAttributesSchema, firefoxFixtureAnnosSchema,
		firefoxFixtureInsertPlace(1, link("www", "/"), "Home", 42, micros(3*time.Hour)),
		firefoxFixtureInsertPlace(2, link("docs", "/guide?q=go&lang=en"), `Guide, "quoted" & <b>bold</b>`, 7, micros(4*time.Hour)),
		firefoxFixtureInsertPlace(3, link("news", "/ja"), "ニュース", 3, micros(5*time.Hour)),
		firefoxFixtureInsertPlace(4, link("docs", "/"), "Docs", 0, 0),
		firefoxFixtureInsertPlace(5, link("news", "/long-read"), "Long read", 1, micros(9*time.Hour)),
		firefoxFixtureInsertPlace(6, link("files", "/report.pdf"), "", 1, micros(6*time.Hour)),
		firefoxFixtureInsertPlace(7, link("files", "/archive.tar.gz"), "", 1, micros(7*time.Hour)),
		firefoxFixtureInsertPlace(8, link("shop", "/menu"), "Café menu", 2, micros(10*time.Hour)),
		firefoxFixtureInsertPlace(9, link("files", "/setup.exe"), "", 1, micros(7*time.Hour+30*time.Minute)),

		firefoxFixtureInsertBookmark(1, 2, 0, 0, 0, "", "root________", micros(0)),
		firefoxFixtureInsertBookmark(2, 2, 0, 1, 0, "menu", "menu________", micros(0)),
		firefoxFixtureInsertBookmark(3, 2, 0, 1, 1, "toolbar", "toolbar_____", micros(0)),
		firefoxFixtureInsertBookmark(4, 2, 0, 1, 2, "tags", "tags________", micros(0)),
		firefoxFixtureInsertBookmark(5, 2, 0, 1, 3, "unfiled", "unfiled_____", micros(0)),
		firefoxFixtureInsertBookmark(6, 2, 0, 1, 4, "mobile", "mobile______", micros(0)),
		firefoxFixtureInsertBookmark(7, 1, 4, 3, 0, "Docs", "bookmark0007", micros(8*time.Hour)),
		firefoxFixtureInsertBookmark(8, 2, 0, 3, 1, "Reading", "folder000008", micros(0)),
		firefoxFixtureInsertBookmark(9, 1, 5, 8, 0, "Long read <with> & tags", "bookmark0009", micros(9*time.Hour)),
		firefoxFixtureInsertBookmark(10, 3, 0, 3, 2, "", "separator010", micros(0)),
		firefoxFixtureInsertBookmark(11, 1, 8, 2, 0, "Café menu", "bookmark0011", micros(10*time.Hour)),
		firefoxFixtureInsertBookmark(12, 1, 1, 5, 0, "", "bookmark0012", micros(11*time.Hour)),

		`INSERT INTO moz_anno_attributes (id, name) VALUES (1, 'downloads/destinationFileURI'), (2, 'downloads/metaData')`,
		firefoxFixtureInsertAnno(6, 1, "file:///C:/Users/alice/Downloads/report.pdf", micros(6*time.Hour)),
		firefoxFixtureInsertAnno(6, 2, fmt.Sprintf(`{"state":1,"deleted":false,"endTime":%d,"fileSize":52344}`,
			millis(6*time.Hour+3*time.Second)), micros(6*time.Hour)),
		firefoxFixtureInsertAnno(7, 1, "file:///home/alice/Downloads/archive.tar.gz", micros(7*time.Hour)),
		firefoxFixtureInsertAnno(7, 2, fmt.Sprintf(`{"state":1,"deleted":false,"endTime":%d,"fileSize":1048576}`,
			millis(7*time.Hour+time.Minute)), micros(7*time.Hour)),
		firefoxFixtureInsertAnno(9, 1, "file:///Users/alice/Downloads/setup.exe", micros(7*time.Hour+30*time.Minute)),
	)

	fixtureWrite(t, filepath.Join(dir, "extensions.json"), firefoxFixtureExtensions)

	fixtureSQLite(t, filepath.Join(dir, "webappsstore.sqlite"), firefoxFixtureWebappsstoreSchema,
		firefoxFixtureInsertStorage(firefoxFixtureOriginKey("https", "app."+site, 443), "greeting", "café"),
		firefoxFixtureInsertStorage(firefoxFixtureOriginKey("https", "app."+site, 443), "言語", "日本語"),
		firefoxFixtureInsertStorage(firefoxFixtureOriginKey("http", "shop."+site, 8080), "cart", `{"items":2}`),
	)
}

// The template goes through the decoder's own parser and Encrypt, so only the ASN.1 layout is spelled out here.
func firefoxFixtureSeal(t *testing.T, marshal func() ([]byte, error), ciphertext *[]byte, key, plaintext []byte) []byte {
	t.Helper()
	template, err := marshal()
	require.NoError(t, err)
	pbe, err := crypto.NewASN1PBE(template)
	require.NoError(t, err)
	*ciphertext, err = pbe.Encrypt(key, plaintext)
	require.NoError(t, err)
	sealed, err := marshal()
	require.NoError(t, err)
	return sealed
}

type firefoxFixturePBES2 struct {
	Algorithm struct {
		OID    asn1.ObjectIdentifier
		Params struct {
			KDF struct {
				OID    asn1.ObjectIdentifier
				Params struct {
					Salt       []byte
					Iterations int
					KeyLength  int
					PRF        struct{ OID asn1.ObjectIdentifier }
				}
			}
			Cipher struct {
				OID asn1.ObjectIdentifier
				IV  []byte
			}
		}
	}
	Ciphertext []byte
}

func firefoxFixturePBES2Seal(t *testing.T, globalSalt []byte, seed string, plaintext []byte) []byte {
	t.Helper()
	var pbe firefoxFixturePBES2
	pbe.Algorithm.OID = oidPBES2
	kdf := &pbe.Algorithm.Params.KDF
	kdf.OID = oidPBKDF2
	kdf.Params.Salt = firefoxFixtureBytes(seed+"/salt", 32)
	kdf.Params.Iterations = 1
	kdf.Params.KeyLength = 32
	kdf.Params.PRF.OID = oidHMACWithSHA256
	pbe.Algorithm.Params.Cipher.OID = oidAES256CBC
	pbe.Algorithm.Params.Cipher.IV = firefoxFixtureBytes(seed+"/iv", 14)
	return firefoxFixtureSeal(t, func() ([]byte, error) { return asn1.Marshal(pbe) }, &pbe.Ciphertext, globalSalt, plaintext)
}

type firefoxFixtureLoginPBE struct {
	KeyID  []byte
	Cipher struct {
		OID asn1.ObjectIdentifier
		IV  []byte
	}
	Ciphertext []byte
}

func firefoxFixtureLoginSeal(t *testing.T, key []byte, seed, plaintext string) string {
	t.Helper()
	pbe := firefoxFixtureLoginPBE{KeyID: firefoxFixtureKeyID}
	if len(key) == 24 {
		pbe.Cipher.OID = oidDESEDE3CBC
		pbe.Cipher.IV = firefoxFixtureBytes(seed, 8)
	} else {
		pbe.Cipher.OID = oidAES256CBC
		pbe.Cipher.IV = firefoxFixtureBytes(seed, 16)
	}
	sealed := firefoxFixtureSeal(t, func() ([]byte, error) { return asn1.Marshal(pbe) }, &pbe.Ciphertext, key, []byte(plaintext))
	return base64.StdEncoding.EncodeToString(sealed)
}

func firefoxFixtureBytes(seed string, n int) []byte {
	sum := sha256.Sum256([]byte(seed))
	return sum[:n]
}

// webappsstore2 stores the host reversed, so a domain and its subdomains share a key prefix.
func firefoxFixtureOriginKey(scheme, host string, port int) string {
	reversed := []byte(host)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return fmt.Sprintf("%s.:%s:%d", reversed, scheme, port)
}

func firefoxFixtureText(s string) string {
	if s == "" {
		return "NULL"
	}
	return "'" + s + "'"
}

func firefoxFixtureInsertCookie(name, value, host, path string, expiry, creationTime int64, secure, httpOnly, sameSite int) string {
	return fmt.Sprintf(
		`INSERT INTO moz_cookies (originAttributes, name, value, host, path, expiry, lastAccessed, creationTime,
		 isSecure, isHttpOnly, sameSite, rawSameSite, schemeMap)
		 VALUES ('', '%s', '%s', '%s', '%s', %d, %d, %d, %d, %d, %d, %d, 2)`,
		name, value, host, path, expiry, creationTime, creationTime, secure, httpOnly, sameSite, sameSite,
	)
}

func firefoxFixtureInsertPlace(id int, url, title string, visitCount int, lastVisitDate int64) string {
	lastVisit := "NULL"
	if lastVisitDate != 0 {
		lastVisit = fmt.Sprint(lastVisitDate)
	}
	return fmt.Sprintf(
		`INSERT INTO moz_places (id, url, title, rev_host, visit_count, last_visit_date, guid, url_hash)
		 VALUES (%d, '%s', %s, '', %d, %s, 'place%07d', 0)`,
		id, url, firefoxFixtureText(title), visitCount, lastVisit, id,
	)
}

func firefoxFixtureInsertBookmark(id, bookmarkType, fk, parent, position int, title, guid string, dateAdded int64) string {
	fkValue := "NULL"
	if fk != 0 {
		fkValue = fmt.Sprint(fk)
	}
	return fmt.Sprintf(
		`INSERT INTO moz_bookmarks (id, type, fk, parent, position, title, dateAdded, lastModified, guid)
		 VALUES (%d, %d, %s, %d, %d, %s, %d, %d, '%s')`,
		id, bookmarkType, fkValue, parent, position, firefoxFixtureText(title), dateAdded, dateAdded, guid,
	)
}

func firefoxFixtureInsertAnno(placeID, attributeID int, content string, dateAdded int64) string {
	return fmt.Sprintf(
		`INSERT INTO moz_annos (place_id, anno_attribute_id, content, flags, expiration, type, dateAdded, lastModified)
		 VALUES (%d, %d, '%s', 0, 4, 3, %d, %d)`,
		placeID, attributeID, content, dateAdded, dateAdded,
	)
}

func firefoxFixtureInsertStorage(originKey, key, value string) string {
	return fmt.Sprintf(
		`INSERT INTO webappsstore2 (originAttributes, originKey, scope, key, value) VALUES ('', '%s', '', '%s', '%s')`,
		originKey, key, value,
	)
}

const firefoxFixtureMetaDataSchema = `CREATE TABLE metaData (id PRIMARY KEY UNIQUE ON CONFLICT REPLACE, item1, item2)`

const firefoxFixtureNSSPrivateSchema = `CREATE TABLE nssPrivate (id PRIMARY KEY UNIQUE ON CONFLICT ABORT, a11, a102)`

const firefoxFixtureCookiesSchema = `CREATE TABLE moz_cookies (
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

const firefoxFixturePlacesSchema = `CREATE TABLE moz_places (
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

const firefoxFixtureBookmarksSchema = `CREATE TABLE moz_bookmarks (
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

const firefoxFixtureAnnoAttributesSchema = `CREATE TABLE moz_anno_attributes (
	id INTEGER PRIMARY KEY,
	name VARCHAR(32) UNIQUE NOT NULL
)`

const firefoxFixtureAnnosSchema = `CREATE TABLE moz_annos (
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

const firefoxFixtureWebappsstoreSchema = `CREATE TABLE webappsstore2 (
	originAttributes TEXT,
	originKey TEXT,
	scope TEXT,
	key TEXT,
	value TEXT
)`

const firefoxFixtureLogins = `{
  "nextId": 4,
  "logins": [
    {
      "id": 1,
      "hostname": "https://accounts.%[1]s",
      "httpRealm": null,
      "formSubmitURL": "https://login.%[1]s",
      "usernameField": "username",
      "passwordField": "password",
      "encryptedUsername": "%[2]s",
      "encryptedPassword": "%[3]s",
      "encType": 1,
      "timeCreated": %[4]d,
      "timesUsed": 3
    },
    {
      "id": 2,
      "hostname": "https://shop.%[1]s",
      "httpRealm": null,
      "formSubmitURL": "",
      "usernameField": "email",
      "passwordField": "password",
      "encryptedUsername": "%[5]s",
      "encryptedPassword": "%[6]s",
      "encType": 1,
      "timeCreated": %[7]d,
      "timesUsed": 1
    },
    {
      "id": 3,
      "hostname": "https://intranet.%[1]s",
      "httpRealm": "Staff only",
      "formSubmitURL": null,
      "usernameField": "",
      "passwordField": "",
      "encryptedUsername": "%[8]s",
      "encryptedPassword": "%[9]s",
      "encType": 1,
      "timeCreated": %[10]d,
      "timesUsed": 7
    }
  ],
  "potentiallyVulnerablePasswords": [],
  "dismissedBreachAlertsByLoginGUID": {},
  "version": 3
}`

const firefoxFixtureExtensions = `{
  "schemaVersion": 36,
  "addons": [
    {
      "id": "tab-organizer@tabs.example",
      "version": "2.4.1",
      "type": "extension",
      "defaultLocale": {
        "name": "Tab Organizer",
        "description": "Groups tabs by site, \"neatly\" & <fast>",
        "creator": "Tabs Example",
        "homepageURL": "https://tabs.example/"
      },
      "active": true,
      "userDisabled": false,
      "location": "app-profile"
    },
    {
      "id": "{4b5ac9d3-1f2e-4d6c-9a8b-7c6d5e4f3a2b}",
      "version": "0.9.0",
      "type": "extension",
      "defaultLocale": {"name": "Reader Mode", "description": "Distraction-free reading", "homepageURL": null},
      "active": false,
      "userDisabled": true,
      "location": "app-profile"
    },
    {
      "id": "formautofill@mozilla.org",
      "version": "1.0.1",
      "type": "extension",
      "defaultLocale": {"name": "Form Autofill", "description": "Autofill forms with saved profiles.", "homepageURL": null},
      "active": true,
      "location": "app-system-defaults"
    },
    {
      "id": "default-theme@mozilla.org",
      "version": "1.3",
      "type": "theme",
      "defaultLocale": {"name": "System theme — auto", "description": "Follow the operating system setting.", "homepageURL": null},
      "active": true,
      "location": "app-builtin"
    }
  ]
}`
