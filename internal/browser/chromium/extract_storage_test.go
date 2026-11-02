package chromium

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func TestDecodeChromiumString(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    string
		wantErr string
	}{
		{
			name:  "latin1 ascii",
			input: testEncodeLatin1("abc123"),
			want:  "abc123",
		},
		{
			name:  "latin1 non-ascii",
			input: append([]byte{chromiumStringLatin1Format}, 0x6E, 0x61, 0xEF, 0x76, 0x65), // "naïve" in Latin-1
			want:  "na\u00efve",                                                             // U+00EF = ï
		},
		{
			name:  "utf16le ascii",
			input: testEncodeUTF16("hello"),
			want:  "hello",
		},
		{
			name:  "utf16le japanese",
			input: testEncodeUTF16("テスト"),
			want:  "テスト",
		},
		{
			name:  "utf16le empty content",
			input: []byte{chromiumStringUTF16Format},
			want:  "",
		},
		{
			name:    "unknown format",
			input:   []byte{2, 'x'},
			wantErr: "unknown chromium string format",
		},
		{
			name:    "invalid utf16 byte length",
			input:   []byte{chromiumStringUTF16Format, 0x61},
			wantErr: "invalid UTF-16 byte length",
		},
		{
			name:    "empty input",
			input:   []byte{},
			wantErr: "empty chromium string",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := decodeChromiumString(testCase.input)
			if testCase.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestParseLocalStorageEntry(t *testing.T) {
	tests := []struct {
		name       string
		key        []byte
		value      []byte
		wantParsed bool
		wantMeta   bool
		wantURL    string
		wantKey    string
		wantValue  string
	}{
		{
			name:       "skip VERSION",
			key:        []byte(localStorageVersionKey),
			wantParsed: false,
		},
		{
			name:       "META entry",
			key:        []byte(localStorageMetaPrefix + "https://example.com"),
			value:      []byte{0x08, 0x96, 0x01},
			wantParsed: true,
			wantMeta:   true,
			wantURL:    "https://example.com",
			wantValue:  "meta data, value bytes is [8 150 1]",
		},
		{
			name:       "METAACCESS entry",
			key:        []byte(localStorageMetaAccessKey + "https://example.com"),
			value:      []byte{0x10, 0x20},
			wantParsed: true,
			wantMeta:   true,
			wantURL:    "https://example.com",
			wantValue:  "meta data, value bytes is [16 32]",
		},
		{
			name:       "latin1 data entry",
			key:        append([]byte("_https://example.com\x00"), testEncodeLatin1("token")...),
			value:      testEncodeLatin1("abc123"),
			wantParsed: true,
			wantURL:    "https://example.com",
			wantKey:    "token",
			wantValue:  "abc123",
		},
		{
			name:       "utf16 data entry",
			key:        append([]byte("_https://example.com\x00"), testEncodeUTF16("テスト")...),
			value:      testEncodeUTF16("データ"),
			wantParsed: true,
			wantURL:    "https://example.com",
			wantKey:    "テスト",
			wantValue:  "データ",
		},
		{
			name:       "missing origin separator",
			key:        []byte("_https://example.com"),
			value:      testEncodeLatin1("abc123"),
			wantParsed: true,
			wantURL:    "",
			wantKey:    "",
			wantValue:  "abc123",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			entry, parsed := parseLocalStorageEntry(testCase.key, testCase.value)
			assert.Equal(t, testCase.wantParsed, parsed)
			if !parsed {
				return
			}
			assert.Equal(t, testCase.wantMeta, entry.IsMeta)
			assert.Equal(t, testCase.wantURL, entry.URL)
			assert.Equal(t, testCase.wantKey, entry.Key)
			assert.Equal(t, testCase.wantValue, entry.Value)
		})
	}
}

func setupLocalStorageLevelDB(t *testing.T) string {
	t.Helper()
	return createTestLevelDB(t, map[string]string{
		localStorageVersionKey:                                                           "1",
		localStorageMetaPrefix + "https://example.com":                                   string([]byte{0x08, 0x96, 0x01}),
		localStorageMetaAccessKey + "https://example.com":                                string([]byte{0x10, 0x20}),
		string(append([]byte("_https://example.com\x00"), testEncodeLatin1("token")...)): string(testEncodeLatin1("abc123")),
		string(append([]byte("_https://example.com\x00"), testEncodeUTF16("テスト")...)):    string(testEncodeUTF16("データ")),
	})
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

func TestExtractLocalStorage(t *testing.T) {
	dir := setupLocalStorageLevelDB(t)

	var out testCollector
	err := extractLocalStorage(dir, &out)
	got := collected[record.LocalStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 4, "VERSION filtered, META kept, data kept")

	metaCount := 0
	byKey := map[string]string{}
	for _, entry := range got {
		assert.Equal(t, "https://example.com", entry.URL)
		if entry.IsMeta {
			metaCount++
			assert.Contains(t, entry.Value, "meta data, value bytes is")
			continue
		}
		byKey[entry.Key] = entry.Value
	}
	assert.Equal(t, 2, metaCount)
	assert.Equal(t, "abc123", byKey["token"])
	assert.Equal(t, "データ", byKey["テスト"])
}

func TestExtractSessionStorage(t *testing.T) {
	dir := setupSessionStorageLevelDB(t)

	var out testCollector
	err := extractSessionStorage(dir, &out)
	got := collected[record.SessionStorage](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byKey := map[string]string{}
	for _, entry := range got {
		byKey[entry.URL+"/"+entry.Key] = entry.Value
	}
	assert.Equal(t, "false", byKey["https://github.com//__darkreader__wasEnabledForHost"])
	assert.Equal(t, "abc123", byKey["https://example.com//token"])
}

// TestExtractStorage_LeavesTheCopyUnchanged opens read-only, since a journal replay can fail on Windows and lose data.
func TestExtractStorage_LeavesTheCopyUnchanged(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T) string
		extract func(string, browser.Collector) error
	}{
		{name: "localstorage", setup: setupLocalStorageLevelDB, extract: extractLocalStorage},
		{name: "sessionstorage", setup: setupSessionStorageLevelDB, extract: extractSessionStorage},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			dir := testCase.setup(t)
			before := levelDBFiles(t, dir)
			require.NoError(t, testCase.extract(dir, &testCollector{}))
			assert.Equal(t, before, levelDBFiles(t, dir))
		})
	}
}

func levelDBFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
		files[entry.Name()] = string(data)
	}
	return files
}

func testEncodeLatin1(text string) []byte {
	return append([]byte{chromiumStringLatin1Format}, []byte(text)...)
}

func testEncodeUTF16(text string) []byte {
	encoded := utf16.Encode([]rune(text))
	result := make([]byte, 1, 1+len(encoded)*2)
	result[0] = chromiumStringUTF16Format
	for _, unit := range encoded {
		var raw [2]byte
		binary.LittleEndian.PutUint16(raw[:], unit)
		result = append(result, raw[:]...)
	}
	return result
}

func testEncodeUTF16Raw(text string) []byte {
	encoded := utf16.Encode([]rune(text))
	result := make([]byte, 0, len(encoded)*2)
	for _, unit := range encoded {
		var raw [2]byte
		binary.LittleEndian.PutUint16(raw[:], unit)
		result = append(result, raw[:]...)
	}
	return result
}
