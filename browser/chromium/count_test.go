package chromium

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountBookmarks(t *testing.T) {
	path := setupBookmarkJSON(t)

	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count) // 3 URLs, folders not counted
}

func TestCountBookmarks_Empty(t *testing.T) {
	path := createTestJSON(t, "Bookmarks", `{"roots": {}}`)

	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountCookies(t *testing.T) {
	path := setupCookieDB(t)

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountCookies_Empty(t *testing.T) {
	path := createTestDB(t, "Cookies", cookiesSchema)

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountCreditCards(t *testing.T) {
	path := setupCreditCardDB(t)

	count, err := countCreditCards(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountCreditCards_Empty(t *testing.T) {
	path := createTestDB(t, "Web Data", creditCardsSchema)

	count, err := countCreditCards(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountYandexCreditCards(t *testing.T) {
	path := createTestDB(t, "Ya Credit Cards", `CREATE TABLE records (guid TEXT, public_data TEXT, private_data BLOB)`,
		`INSERT INTO records (guid) VALUES ('g1')`,
		`INSERT INTO records (guid) VALUES ('g2')`,
		`INSERT INTO records (guid) VALUES ('g3')`,
	)

	count, err := countYandexCreditCards(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountDownloads(t *testing.T) {
	path := setupDownloadDB(t)

	count, err := countDownloads(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountDownloads_Empty(t *testing.T) {
	path := createTestDB(t, "History", downloadsSchema)

	count, err := countDownloads(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountExtensions(t *testing.T) {
	path := setupExtensionJSON(t)

	count, err := countExtensions(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // system (5) and component (10) skipped
}

func TestCountOperaExtensions(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{
		"extensions": {
			"opsettings": {
				"opera-ext-1": {
					"location": 1,
					"manifest": {"name": "Opera Ad Blocker", "version": "2.0.0"}
				},
				"system-ext": {
					"location": 5,
					"manifest": {"name": "System", "version": "1.0"}
				}
			}
		}
	}`)

	count, err := countOperaExtensions(path)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCountExtensions_Empty(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{
		"extensions": {"settings": {}}
	}`)

	count, err := countExtensions(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountHistories(t *testing.T) {
	path := setupHistoryDB(t)

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountHistories_Empty(t *testing.T) {
	path := createTestDB(t, "History", urlsSchema)

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountPasswords(t *testing.T) {
	path := setupLoginDB(t)

	count, err := countPasswords(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountPasswords_Empty(t *testing.T) {
	path := createTestDB(t, "Login Data", loginsSchema)

	count, err := countPasswords(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountLocalStorage(t *testing.T) {
	dir := setupLocalStorageLevelDB(t)

	count, err := countLocalStorage(dir)
	require.NoError(t, err)
	assert.Equal(t, 4, count) // VERSION filtered, 2 META + 2 data entries kept
}

// ---------------------------------------------------------------------------
// countSessionStorage
// ---------------------------------------------------------------------------

func TestCountSessionStorage(t *testing.T) {
	dir := setupSessionStorageLevelDB(t)

	count, err := countSessionStorage(dir)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // only map- entries with key separator
}

func TestCountSessionStorage_Empty(t *testing.T) {
	dir := createTestLevelDB(t, map[string]string{
		"next-map-id": "1",
		"version":     "1",
	})

	count, err := countSessionStorage(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func setupBookmarkJSON(t *testing.T) string {
	t.Helper()
	return createTestJSON(t, "Bookmarks", `{
		"roots": {
			"bookmark_bar": {
				"name": "Bookmarks Bar",
				"type": "folder",
				"children": [
					{"name": "Go", "type": "url", "url": "https://go.dev", "date_added": "13360000000000000"},
					{
						"name": "News",
						"type": "folder",
						"children": [
							{"name": "HN", "type": "url", "url": "https://news.ycombinator.com", "date_added": "13350000000000000"}
						]
					}
				]
			},
			"other": {
				"name": "Other",
				"type": "folder",
				"children": [
					{"name": "GitHub", "type": "url", "url": "https://github.com", "date_added": "13370000000000000"}
				]
			}
		}
	}`)
}

func setupCookieDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "Cookies", cookiesSchema,
		insertCookie("session", ".old.com", "/", "", 13340000000000000, 13350000000000000, 1, 1),
		insertCookie("token", ".new.com", "/api", "", 13360000000000000, 13370000000000000, 1, 0),
	)
}

func setupCreditCardDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "Web Data", creditCardsSchema,
		insertCreditCard("John Doe", 12, 2025, "", "Johnny", "addr-1"),
		insertCreditCard("Jane Smith", 6, 2027, "", "", ""),
	)
}

func setupDownloadDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "History", downloadsSchema,
		insertDownload("/tmp/old.zip", "https://old.com/file.zip", "application/zip", 1024, 13340000000000000, 13340000100000000),
		insertDownload("/tmp/new.pdf", "https://new.com/doc.pdf", "application/pdf", 2048, 13360000000000000, 13360000200000000),
	)
}

func setupExtensionJSON(t *testing.T) string {
	t.Helper()
	return createTestJSON(t, "Secure Preferences", `{
		"extensions": {
			"settings": {
				"abc123": {
					"location": 1,
					"manifest": {
						"name": "React DevTools",
						"description": "React debugging",
						"version": "4.28.0"
					}
				},
				"system-ext": {
					"location": 5,
					"manifest": {"name": "System", "version": "1.0"}
				},
				"component-ext": {
					"location": 10,
					"manifest": {"name": "Component", "version": "1.0"}
				},
				"def456": {
					"location": 1,
					"manifest": {
						"name": "uBlock Origin",
						"description": "Ad blocker",
						"version": "1.52.0"
					}
				}
			}
		}
	}`)
}

func setupHistoryDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "History", urlsSchema,
		insertURL("https://github.com", "GitHub", 100, 13370000000000000),
		insertURL("https://go.dev", "Go Dev", 50, 13360000000000000),
		insertURL("https://example.com", "Example", 200, 13350000000000000),
	)
}

func setupLocalStorageLevelDB(t *testing.T) string {
	t.Helper()
	return createTestLevelDB(t, map[string]string{
		"VERSION":                             "1",
		"META:" + "https://example.com":       string([]byte{0x08, 0x96, 0x01}),
		"METAACCESS:" + "https://example.com": string([]byte{0x10, 0x20}),
		string(append([]byte("_https://example.com\x00"), testEncodeLatin1("token")...)): string(testEncodeLatin1("abc123")),
		string(append([]byte("_https://example.com\x00"), testEncodeUTF16("テスト")...)):    string(testEncodeUTF16("データ")),
	})
}

func setupLoginDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "Login Data", loginsSchema,
		insertLogin("https://old.com", "https://old.com/login", "alice", 13340000000000000),
		insertLogin("https://new.com", "https://new.com/login", "bob", 13360000000000000),
	)
}

func setupSessionStorageLevelDB(t *testing.T) string {
	t.Helper()
	return createTestLevelDB(t, map[string]string{
		"namespace-abcd1234_5678_9abc_def0_111111111111-https://github.com/":  "100",
		"namespace-abcd1234_5678_9abc_def0_111111111111-https://example.com/": "101",
		"map-100-__darkreader__wasEnabledForHost":                             string(testEncodeUTF16Raw("false")),
		"map-101-token": string(testEncodeUTF16Raw("abc123")),
		"next-map-id":   "200",
		"version":       "1",
	})
}

func testEncodeLatin1(s string) []byte {
	return append([]byte{1}, []byte(s)...)
}

func testEncodeUTF16(s string) []byte {
	encoded := utf16.Encode([]rune(s))
	result := make([]byte, 1, 1+len(encoded)*2)
	result[0] = 0
	for _, r := range encoded {
		var raw [2]byte
		binary.LittleEndian.PutUint16(raw[:], r)
		result = append(result, raw[:]...)
	}
	return result
}

// testEncodeUTF16Raw encodes as raw UTF-16 LE without format byte prefix
// (used by session storage values).
func testEncodeUTF16Raw(s string) []byte {
	encoded := utf16.Encode([]rune(s))
	result := make([]byte, 0, len(encoded)*2)
	for _, r := range encoded {
		var raw [2]byte
		binary.LittleEndian.PutUint16(raw[:], r)
		result = append(result, raw[:]...)
	}
	return result
}
