package chromium

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// decodeChromiumString
// ---------------------------------------------------------------------------

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeChromiumString(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// parseLocalStorageEntry
// ---------------------------------------------------------------------------

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, parsed := parseLocalStorageEntry(tt.key, tt.value)
			assert.Equal(t, tt.wantParsed, parsed)
			if !parsed {
				return
			}
			assert.Equal(t, tt.wantMeta, entry.IsMeta)
			assert.Equal(t, tt.wantURL, entry.URL)
			assert.Equal(t, tt.wantKey, entry.Key)
			assert.Equal(t, tt.wantValue, entry.Value)
		})
	}
}

// ---------------------------------------------------------------------------
// extractLocalStorage (integration with LevelDB)
// ---------------------------------------------------------------------------

func TestExtractLocalStorage(t *testing.T) {
	dir := createTestLevelDB(t, map[string]string{
		localStorageVersionKey:                                                           "1",
		localStorageMetaPrefix + "https://example.com":                                   string([]byte{0x08, 0x96, 0x01}),
		localStorageMetaAccessKey + "https://example.com":                                string([]byte{0x10, 0x20}),
		string(append([]byte("_https://example.com\x00"), testEncodeLatin1("token")...)): string(testEncodeLatin1("abc123")),
		string(append([]byte("_https://example.com\x00"), testEncodeUTF16("テスト")...)):    string(testEncodeUTF16("データ")),
	})

	got, err := extractLocalStorage(dir)
	require.NoError(t, err)
	require.Len(t, got, 4, "VERSION filtered, META kept, data kept")

	metaCount := 0
	byKey := map[string]string{}
	for _, e := range got {
		assert.Equal(t, "https://example.com", e.URL)
		if e.IsMeta {
			metaCount++
			assert.Contains(t, e.Value, "meta data, value bytes is")
			continue
		}
		byKey[e.Key] = e.Value
	}
	assert.Equal(t, 2, metaCount)
	assert.Equal(t, "abc123", byKey["token"])
	assert.Equal(t, "データ", byKey["テスト"])
}

// ---------------------------------------------------------------------------
// extractSessionStorage
// ---------------------------------------------------------------------------

func TestExtractSessionStorage(t *testing.T) {
	dir := createTestLevelDB(t, map[string]string{
		// Namespace entry: maps guid+origin → map_id
		"namespace-abcd1234_5678_9abc_def0_111111111111-https://github.com/":  "100",
		"namespace-abcd1234_5678_9abc_def0_111111111111-https://example.com/": "101",
		// Map entries: actual data (values are raw UTF-16 LE)
		"map-100-__darkreader__wasEnabledForHost": string(testEncodeUTF16Raw("false")),
		"map-101-token": string(testEncodeUTF16Raw("abc123")),
		// Metadata: should be skipped
		"next-map-id": "200",
		"version":     "1",
	})

	got, err := extractSessionStorage(dir)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byKey := map[string]string{}
	for _, entry := range got {
		byKey[entry.URL+"/"+entry.Key] = entry.Value
	}
	assert.Equal(t, "false", byKey["https://github.com//__darkreader__wasEnabledForHost"])
	assert.Equal(t, "abc123", byKey["https://example.com//token"])
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testEncodeLatin1(s string) []byte {
	return append([]byte{chromiumStringLatin1Format}, []byte(s)...)
}

func testEncodeUTF16(s string) []byte {
	encoded := utf16.Encode([]rune(s))
	result := make([]byte, 1, 1+len(encoded)*2)
	result[0] = chromiumStringUTF16Format
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
