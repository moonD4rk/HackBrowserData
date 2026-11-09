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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func TestReadOriginBlock_FirstParty(t *testing.T) {
	data := encodeOriginFile("https://example.com", "https://example.com")
	top, position, err := readOriginBlock(data, 0)
	require.NoError(t, err)
	assert.Equal(t, "https", top.scheme)
	assert.Equal(t, "example.com", top.host)
	assert.Equal(t, uint16(0), top.port, "port 0 ⇒ scheme default")

	frame, _, err := readOriginBlock(data, position)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", formatOriginURL(frame))
}

func TestReadOriginBlock_NonDefaultPort(t *testing.T) {
	data := encodeOriginFile("https://example.com:8443", "https://example.com:8443")
	top, _, err := readOriginBlock(data, 0)
	require.NoError(t, err)
	assert.Equal(t, uint16(8443), top.port)
	assert.Equal(t, "https://example.com:8443", formatOriginURL(top))
}

func TestReadOriginBlock_Latin1HighByte(t *testing.T) {
	// Scheme and host records are Latin-1 (0x01), so é is 0xE9, not invalid UTF-8 passed through.
	data := []byte{
		0x04, 0x00, 0x00, 0x00, 0x01, 'h', 't', 't', 'p', // scheme "http"
		0x04, 0x00, 0x00, 0x00, 0x01, 'c', 'a', 'f', 0xe9, // host "café" (Latin-1)
		0x00, // port default
	}
	endpoint, _, err := readOriginBlock(data, 0)
	require.NoError(t, err)
	assert.Equal(t, "http", endpoint.scheme)
	assert.Equal(t, "café", endpoint.host)
}

func TestDecodeLatin1(t *testing.T) {
	assert.Equal(t, "café", decodeLatin1([]byte{'c', 'a', 'f', 0xe9}))
	assert.Equal(t, "hello", decodeLatin1([]byte("hello")))
	assert.Empty(t, decodeLatin1(nil))
}

func TestReadOriginFile_FramePreferred(t *testing.T) {
	dir := t.TempDir()
	originPath := filepath.Join(dir, "origin")
	require.NoError(t, os.WriteFile(originPath,
		encodeOriginFile("https://top.example.com", "https://iframe.example.com"), 0o644))

	got, err := readOriginFile(originPath)
	require.NoError(t, err)
	assert.Equal(t, "https://iframe.example.com", got)
}

func TestReadOriginFile_FallbackToTop(t *testing.T) {
	// Only the top-frame block is written; the extractor falls back to it.
	var data []byte
	data = appendOriginBlock(data, "https://example.com")
	originPath := filepath.Join(t.TempDir(), "origin")
	require.NoError(t, os.WriteFile(originPath, data, 0o644))

	got, err := readOriginFile(originPath)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", got)
}

func TestReadOriginFile_Malformed(t *testing.T) {
	originPath := filepath.Join(t.TempDir(), "origin")
	require.NoError(t, os.WriteFile(originPath, []byte{0x01, 0x02}, 0o644))

	_, err := readOriginFile(originPath)
	require.Error(t, err)
}

func TestDecodeUTF16LE(t *testing.T) {
	t.Run("ascii", func(t *testing.T) {
		assert.Equal(t, "hello", decodeUTF16LE(encodeUTF16LE("hello")))
	})
	t.Run("cjk", func(t *testing.T) {
		assert.Equal(t, "你好世界", decodeUTF16LE(encodeUTF16LE("你好世界")))
	})
	t.Run("mixed", func(t *testing.T) {
		assert.Equal(t, "hello 世界 🌍", decodeUTF16LE(encodeUTF16LE("hello 世界 🌍")))
	})
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, decodeUTF16LE(nil))
		assert.Empty(t, decodeUTF16LE([]byte{}))
	})
	t.Run("odd length falls back to raw string", func(t *testing.T) {
		assert.Equal(t, "abc", decodeUTF16LE([]byte("abc")))
	})
}

func TestDecodeLocalStorageValue_Truncates(t *testing.T) {
	// 1100 chars × 2 bytes = 2200 bytes, over the 2048 cap.
	oversized := encodeUTF16LE(strings.Repeat("x", 1100))
	got := decodeLocalStorageValue(oversized)
	assert.Contains(t, got, "too long")
	assert.Contains(t, got, "2048")

	assert.Contains(t, decodeLocalStorageValue(encodeUTF16LE(strings.Repeat("x", 1024))), "length is 2048", "the cap itself is too long")
	assert.Equal(t, strings.Repeat("x", 1023), decodeLocalStorageValue(encodeUTF16LE(strings.Repeat("x", 1023))))
}

func TestExtractLocalStorage_SingleOrigin(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {{Key: "auth_token", Value: "abc123"}},
	})
	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://example.com", entries[0].URL)
	assert.Equal(t, "auth_token", entries[0].Key)
	assert.Equal(t, "abc123", entries[0].Value)
	assert.False(t, entries[0].IsMeta)
}

func TestExtractLocalStorage_MultiOrigin(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://github.com": {
			{Key: "theme", Value: "dark"},
			{Key: "lang", Value: "en"},
		},
		"https://example.com:8443": {
			{Key: "session", Value: "xyz"},
		},
	})
	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	byURL := make(map[string][]string)
	for _, entry := range entries {
		byURL[entry.URL] = append(byURL[entry.URL], entry.Key+"="+entry.Value)
	}
	assert.ElementsMatch(t, []string{"theme=dark", "lang=en"}, byURL["https://github.com"])
	assert.ElementsMatch(t, []string{"session=xyz"}, byURL["https://example.com:8443"])
}

func TestExtractLocalStorage_CJKAndEmoji(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {
			{Key: "名字", Value: "张三"},
			{Key: "status", Value: "hello 世界 🌍"},
		},
	})
	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	values := make(map[string]string)
	for _, entry := range entries {
		values[entry.Key] = entry.Value
	}
	assert.Equal(t, "张三", values["名字"])
	assert.Equal(t, "hello 世界 🌍", values["status"])
}

func TestExtractLocalStorage_EmptyItemTable(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": nil,
	})
	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExtractLocalStorage_TruncatesOversizedValue(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {{Key: "big", Value: strings.Repeat("x", 1100)}},
	})
	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Value, "too long")
}

func TestExtractLocalStorage_Partitioned(t *testing.T) {
	// Manually construct a partitioned third-party entry: YouTube iframe inside Google top-frame.
	root := filepath.Join(t.TempDir(), "Origins")
	require.NoError(t, os.MkdirAll(root, 0o755))
	writeTestOriginStore(t, root, "topHash", "frameHash",
		"https://accounts.google.com", "https://accounts.youtube.com",
		[]testLocalStorageItem{{Key: "yt-session", Value: "embedded"}},
	)

	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://accounts.youtube.com", entries[0].URL, "frame origin preferred over top-frame")
}

func TestExtractLocalStorage_SkipsFrameDirsWithoutDB(t *testing.T) {
	// Partition dirs with only an origin file are common (cookies-only) and must not fail the extraction.
	root := filepath.Join(t.TempDir(), "Origins")
	frameDir := filepath.Join(root, "topHash", "frameHash")
	require.NoError(t, os.MkdirAll(frameDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(frameDir, "origin"),
		encodeOriginFile("https://example.com", "https://example.com"), 0o644))

	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Empty(t, out.fails)
}

func TestExtractLocalStorage_DirMissing(t *testing.T) {
	err := extractLocalStorage(filepath.Join(t.TempDir(), "does-not-exist"), &testCollector{})
	require.Error(t, err)
}

func TestExtractLocalStorage_EmptyRoot(t *testing.T) {
	var out testCollector
	err := extractLocalStorage(t.TempDir(), &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExtractLocalStorage_ReportsBrokenOriginsAndGoesOn(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Origins")
	for _, hash := range []string{"badDB", "badOrigin", "good"} {
		writeTestOriginStore(t, root, hash, hash, "https://"+hash+".example", "https://"+hash+".example",
			[]testLocalStorageItem{{Key: "k", Value: "v"}})
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "badOrigin", "badOrigin", webkitOriginFile), []byte{0xff}, 0o644))
	badDB := filepath.Join(root, "badDB", "badDB", webkitLocalStorageSubdir, webkitLocalStorageDB)
	require.NoError(t, os.WriteFile(badDB, []byte("not a database"), 0o644))

	var out testCollector
	require.NoError(t, extractLocalStorage(root, &out))
	entries := collected[record.LocalStorage](t, &out)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://good.example", entries[0].URL)
	assert.Len(t, out.fails, 2, "each broken origin is reported, not skipped silently")
}

func TestExtractOriginStorage_ReportsNullKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ls.sqlite3")
	writeLocalStorageDB(t, dbPath, []testLocalStorageItem{
		{Key: "real", Value: "keeper"},
	}, true /*addNullKey*/)

	var out testCollector
	require.NoError(t, extractOriginStorage(dbPath, "https://example.com", &out))
	entries := collected[record.LocalStorage](t, &out)
	require.Len(t, entries, 1)
	assert.Equal(t, "real", entries[0].Key)
	assert.Equal(t, "keeper", entries[0].Value)
	assert.Len(t, out.fails, 1)
}

func TestExtractOriginStorage_EmitsRowsInKeyOrder(t *testing.T) {
	// Rows go in reverse order; ORDER BY key, rowid must return them ascending so exports are deterministic.
	dbPath := filepath.Join(t.TempDir(), "ls.sqlite3")
	writeLocalStorageDB(t, dbPath, []testLocalStorageItem{
		{Key: "zebra", Value: "z"},
		{Key: "mango", Value: "m"},
		{Key: "apple", Value: "a"},
	}, false /*addNullKey*/)

	var out testCollector
	require.NoError(t, extractOriginStorage(dbPath, "https://example.com", &out))
	entries := collected[record.LocalStorage](t, &out)
	require.Len(t, entries, 3)
	assert.Equal(t, "apple", entries[0].Key)
	assert.Equal(t, "mango", entries[1].Key)
	assert.Equal(t, "zebra", entries[2].Key)
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
	encoded = append(encoded, originEncodingASCII)
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

// writeLocalStorageDB inserts a NULL-key row first when addNullKey is set, which extractOriginStorage reports as a failure.
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
