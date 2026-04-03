package localstorage

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestDecodeChromiumString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		want    string
		wantErr string
	}{
		{
			name:  "latin1",
			input: encodeChromiumLatin1("abc123"),
			want:  "abc123",
		},
		{
			name:  "utf16le",
			input: encodeChromiumUTF16("飞连"),
			want:  "飞连",
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
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeChromiumString(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseChromiumLocalStorageEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		key          []byte
		value        []byte
		wantParsed   bool
		wantMeta     bool
		wantURL      string
		wantKey      string
		wantValue    string
		wantContains string
	}{
		{
			name:       "skip version key",
			key:        []byte(chromiumLocalStorageVersionKey),
			wantParsed: false,
		},
		{
			name:       "meta key",
			key:        []byte(chromiumLocalStorageMetaPrefix + "https://example.com"),
			value:      []byte{0x08, 0x96, 0x01},
			wantParsed: true,
			wantMeta:   true,
			wantURL:    "https://example.com",
			wantValue:  "meta data, value bytes is [8 150 1]",
		},
		{
			name:       "meta access key",
			key:        []byte(chromiumLocalStorageMetaAccessKey + "https://example.com"),
			value:      []byte{0x10, 0x20},
			wantParsed: true,
			wantMeta:   true,
			wantURL:    "https://example.com",
			wantValue:  "meta data, value bytes is [16 32]",
		},
		{
			name:       "latin1 business key",
			key:        append([]byte("_https://example.com\x00"), encodeChromiumLatin1("token")...),
			value:      encodeChromiumLatin1("abc123"),
			wantParsed: true,
			wantURL:    "https://example.com",
			wantKey:    "token",
			wantValue:  "abc123",
		},
		{
			name:       "utf16 business key",
			key:        append([]byte("_https://example.com\x00"), encodeChromiumUTF16("飞连")...),
			value:      encodeChromiumUTF16("终端安全"),
			wantParsed: true,
			wantURL:    "https://example.com",
			wantKey:    "飞连",
			wantValue:  "终端安全",
		},
		{
			name:         "unsupported business key format",
			key:          append([]byte("_https://example.com\x00"), []byte{2, 'x'}...),
			value:        encodeChromiumLatin1("abc123"),
			wantParsed:   true,
			wantURL:      "https://example.com",
			wantContains: "unsupported chromium localStorage key encoding",
			wantValue:    "abc123",
		},
		{
			name:         "missing origin separator",
			key:          append([]byte("_https://example.com"), encodeChromiumLatin1("token")...),
			value:        encodeChromiumLatin1("abc123"),
			wantParsed:   true,
			wantContains: "missing origin separator",
			wantValue:    "abc123",
		},
		{
			name:       "unsupported value format",
			key:        append([]byte("_https://example.com\x00"), encodeChromiumLatin1("token")...),
			value:      []byte{2, 'x'},
			wantParsed: true,
			wantURL:    "https://example.com",
			wantKey:    "token",
			wantValue:  "unsupported chromium localStorage value encoding: unknown chromium string format 0x02",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, parsed := parseChromiumLocalStorageEntry(tc.key, tc.value)

			assert.Equal(t, tc.wantParsed, parsed)
			assert.Equal(t, tc.wantMeta, got.IsMeta)
			assert.Equal(t, tc.wantURL, got.URL)
			assert.Equal(t, tc.wantValue, got.Value)
			if tc.wantContains != "" {
				assert.Contains(t, got.Key, tc.wantContains)
				return
			}
			assert.Equal(t, tc.wantKey, got.Key)
		})
	}
}

func TestExtractChromiumLocalStorage(t *testing.T) {
	dir := t.TempDir()
	db, err := leveldb.OpenFile(dir, nil)
	require.NoError(t, err)

	testEntries := map[string][]byte{
		chromiumLocalStorageVersionKey:                                                       []byte("1"),
		chromiumLocalStorageMetaPrefix + "https://example.com":                               {0x08, 0x96, 0x01},
		chromiumLocalStorageMetaAccessKey + "https://example.com":                            {0x10, 0x20},
		string(append([]byte("_https://example.com\x00"), encodeChromiumLatin1("token")...)): encodeChromiumLatin1("abc123"),
		string(append([]byte("_https://example.com\x00"), encodeChromiumUTF16("飞连")...)):     encodeChromiumUTF16("终端安全"),
	}

	for key, value := range testEntries {
		require.NoError(t, db.Put([]byte(key), value, nil))
	}
	require.NoError(t, db.Close())

	got, err := extractChromiumLocalStorage(dir)
	require.NoError(t, err)
	require.Len(t, got, 4)

	metaCount := 0
	valuesByKey := make(map[string]string)
	for _, entry := range got {
		if entry.IsMeta {
			metaCount++
			assert.Equal(t, "https://example.com", entry.URL)
			assert.Contains(t, entry.Value, "meta data, value bytes is")
			continue
		}
		valuesByKey[entry.Key] = entry.Value
		assert.Equal(t, "https://example.com", entry.URL)
	}

	assert.Equal(t, 2, metaCount)
	assert.Equal(t, "abc123", valuesByKey["token"])
	assert.Equal(t, "终端安全", valuesByKey["飞连"])
}

func encodeChromiumLatin1(s string) []byte {
	return append([]byte{chromiumStringLatin1Format}, []byte(s)...)
}

func encodeChromiumUTF16(s string) []byte {
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
