package safari

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/record"
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
	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
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

func TestExtractLocalStorage_SkipsSaltAndStrayFiles(t *testing.T) {
	root := buildTestLocalStorageDir(t, map[string][]testLocalStorageItem{
		"https://example.com": {{Key: "a", Value: "1"}},
	})
	// Drop a "salt" sibling that must not be traversed, plus a stray file at root.
	require.NoError(t, os.WriteFile(filepath.Join(root, "salt"), []byte("pretend salt"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README"), []byte("noise"), 0o644))

	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
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

	var out testCollector
	err := extractLocalStorage(root, &out)
	entries := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	assert.Empty(t, entries)
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

// ---------------------------------------------------------------------------
// NULL-key handling — readLocalStorageFile skips NULL keys.
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
