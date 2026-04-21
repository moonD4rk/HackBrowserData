package safari

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// readOriginBlock / readOriginFile
// ---------------------------------------------------------------------------

func TestReadOriginBlock_FirstParty(t *testing.T) {
	data := encodeOriginFile("https://example.com", "https://example.com")
	top, pos, err := readOriginBlock(data, 0)
	require.NoError(t, err)
	assert.Equal(t, "https", top.scheme)
	assert.Equal(t, "example.com", top.host)
	assert.Equal(t, uint16(0), top.port, "port 0 ⇒ scheme default")

	frame, _, err := readOriginBlock(data, pos)
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
	// WebKit stores scheme/host records with encoding byte 0x01 = Latin-1. Verify high-byte
	// bytes decode as Latin-1 (é = 0xE9) rather than being passed through as invalid UTF-8.
	data := []byte{
		0x04, 0x00, 0x00, 0x00, 0x01, 'h', 't', 't', 'p', // scheme "http"
		0x04, 0x00, 0x00, 0x00, 0x01, 'c', 'a', 'f', 0xe9, // host "café" (Latin-1)
		0x00, // port default
	}
	ep, _, err := readOriginBlock(data, 0)
	require.NoError(t, err)
	assert.Equal(t, "http", ep.scheme)
	assert.Equal(t, "café", ep.host)
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
	// Write only the top-frame block — no frame follows. Extractor should still succeed by
	// falling back to the top-frame origin.
	var buf []byte
	buf = appendOriginBlock(buf, "https://example.com")
	originPath := filepath.Join(t.TempDir(), "origin")
	require.NoError(t, os.WriteFile(originPath, buf, 0o644))

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

// ---------------------------------------------------------------------------
// decodeUTF16LE / decodeLocalStorageValue
// ---------------------------------------------------------------------------

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
}

// ---------------------------------------------------------------------------
// extractLocalStorage — end-to-end over real nested layout fixtures
// ---------------------------------------------------------------------------

func TestExtractLocalStorage_SingleOrigin(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {{Key: "auth_token", Value: "abc123"}},
	})
	entries, err := extractLocalStorage(root)
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
	entries, err := extractLocalStorage(root)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	byURL := make(map[string][]string)
	for _, e := range entries {
		byURL[e.URL] = append(byURL[e.URL], e.Key+"="+e.Value)
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
	entries, err := extractLocalStorage(root)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	values := make(map[string]string)
	for _, e := range entries {
		values[e.Key] = e.Value
	}
	assert.Equal(t, "张三", values["名字"])
	assert.Equal(t, "hello 世界 🌍", values["status"])
}

func TestExtractLocalStorage_EmptyItemTable(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": nil,
	})
	entries, err := extractLocalStorage(root)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExtractLocalStorage_TruncatesOversizedValue(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {{Key: "big", Value: strings.Repeat("x", 1100)}},
	})
	entries, err := extractLocalStorage(root)
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

	entries, err := extractLocalStorage(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://accounts.youtube.com", entries[0].URL, "frame origin preferred over top-frame")
}

func TestExtractLocalStorage_SkipsSaltAndStrayFiles(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {{Key: "a", Value: "1"}},
	})
	// Drop a "salt" sibling that must not be traversed, plus a stray file at root.
	require.NoError(t, os.WriteFile(filepath.Join(root, "salt"), []byte("pretend salt"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README"), []byte("noise"), 0o644))

	entries, err := extractLocalStorage(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "https://example.com", entries[0].URL)
}

func TestExtractLocalStorage_SkipsFrameDirsWithoutDB(t *testing.T) {
	// Partition dirs that only have "origin" but no LocalStorage/ subdir must not error out —
	// real Safari has plenty of these (cookies-only partitions).
	root := filepath.Join(t.TempDir(), "Origins")
	frameDir := filepath.Join(root, "topHash", "frameHash")
	require.NoError(t, os.MkdirAll(frameDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(frameDir, "origin"),
		encodeOriginFile("https://example.com", "https://example.com"), 0o644))

	entries, err := extractLocalStorage(root)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExtractLocalStorage_DirMissing(t *testing.T) {
	_, err := extractLocalStorage(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}

func TestExtractLocalStorage_EmptyRoot(t *testing.T) {
	entries, err := extractLocalStorage(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// ---------------------------------------------------------------------------
// countLocalStorage
// ---------------------------------------------------------------------------

func TestCountLocalStorage(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://a.com":      {{Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
		"https://b.com":      {{Key: "k3", Value: "v3"}},
		"https://c.com:8443": {{Key: "k4", Value: "v4"}},
	})
	count, err := countLocalStorage(root)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestCountLocalStorage_DirMissing(t *testing.T) {
	count, err := countLocalStorage(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
	assert.Equal(t, 0, count)
}

// ---------------------------------------------------------------------------
// NULL-key handling — readLocalStorageFile / countLocalStorageFile both skip NULL keys,
// keeping count and extract in sync.
// ---------------------------------------------------------------------------

func TestReadLocalStorageFile_SkipsNullKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ls.sqlite3")
	writeLocalStorageDB(t, dbPath, []testLocalStorageItem{
		{Key: "real", Value: "keeper"},
	}, true /*addNullKey*/)

	items, err := readLocalStorageFile(dbPath)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "real", items[0].key)
	assert.Equal(t, "keeper", items[0].value)
}

func TestCountLocalStorageFile_SkipsNullKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ls.sqlite3")
	writeLocalStorageDB(t, dbPath, []testLocalStorageItem{
		{Key: "k1", Value: "v1"},
		{Key: "k2", Value: "v2"},
	}, true /*addNullKey*/)

	count, err := countLocalStorageFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "NULL keys are excluded from count to match extract's skip rule")
}

func TestReadLocalStorageFile_ReturnsRowsInKeyOrder(t *testing.T) {
	// Rows are inserted in reverse alphabetical order; ORDER BY key, rowid in the extractor
	// query must surface them ascending so exports are deterministic across runs.
	dbPath := filepath.Join(t.TempDir(), "ls.sqlite3")
	writeLocalStorageDB(t, dbPath, []testLocalStorageItem{
		{Key: "zebra", Value: "z"},
		{Key: "mango", Value: "m"},
		{Key: "apple", Value: "a"},
	}, false /*addNullKey*/)

	items, err := readLocalStorageFile(dbPath)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "apple", items[0].key)
	assert.Equal(t, "mango", items[1].key)
	assert.Equal(t, "zebra", items[2].key)
}

func TestCountLocalStorageFile_MissingTable(t *testing.T) {
	// Real Safari has origin dirs with LocalStorage/localstorage.sqlite3 but no ItemTable yet
	// (seen during live verification). countLocalStorageFile must surface the error so the
	// caller can log-and-skip rather than counting 0 silently.
	dbPath := filepath.Join(t.TempDir(), "empty.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = countLocalStorageFile(dbPath)
	require.Error(t, err)
}
