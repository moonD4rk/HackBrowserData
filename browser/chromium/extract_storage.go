package chromium

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"unicode/utf16"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/moond4rk/hackbrowserdata/types"
)

// Chromium localStorage LevelDB key prefixes and string format bytes.
// Reference: https://chromium.googlesource.com/chromium/src/+/main/components/services/storage/dom_storage/local_storage_impl.cc
const (
	localStorageVersionKey     = "VERSION"
	localStorageMetaPrefix     = "META:"
	localStorageMetaAccessKey  = "METAACCESS:"
	localStorageDataPrefix     = '_'
	chromiumStringUTF16Format  = 0
	chromiumStringLatin1Format = 1
)

const maxLocalStorageValueLength = 2048

func extractLocalStorage(path string) ([]types.StorageEntry, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("leveldb path not found: %s", path)
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var entries []types.StorageEntry
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		entry, ok := parseLocalStorageEntry(iter.Key(), iter.Value())
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, iter.Error()
}

// parseLocalStorageEntry classifies a LevelDB key/value pair and decodes it.
// Returns false only for VERSION entries. META entries are kept with IsMeta=true.
func parseLocalStorageEntry(key, value []byte) (types.StorageEntry, bool) {
	switch {
	case bytes.Equal(key, []byte(localStorageVersionKey)):
		return types.StorageEntry{}, false
	case bytes.HasPrefix(key, []byte(localStorageMetaAccessKey)):
		return types.StorageEntry{
			IsMeta: true,
			URL:    string(bytes.TrimPrefix(key, []byte(localStorageMetaAccessKey))),
			Value:  fmt.Sprintf("meta data, value bytes is %v", value),
		}, true
	case bytes.HasPrefix(key, []byte(localStorageMetaPrefix)):
		return types.StorageEntry{
			IsMeta: true,
			URL:    string(bytes.TrimPrefix(key, []byte(localStorageMetaPrefix))),
			Value:  fmt.Sprintf("meta data, value bytes is %v", value),
		}, true
	case len(key) > 0 && key[0] == localStorageDataPrefix:
		return parseLocalStorageDataEntry(key[1:], value), true
	default:
		return types.StorageEntry{}, false
	}
}

// parseLocalStorageDataEntry decodes a data entry with format: origin\x00<encoded-key>.
func parseLocalStorageDataEntry(key, value []byte) types.StorageEntry {
	entry := types.StorageEntry{
		Value: decodeLocalStorageValue(value),
	}

	separator := bytes.IndexByte(key, 0)
	if separator < 0 {
		return entry
	}

	entry.URL = string(key[:separator])
	scriptKey, err := decodeChromiumString(key[separator+1:])
	if err != nil {
		return entry
	}
	entry.Key = scriptKey
	return entry
}

// decodeChromiumString decodes a Chromium-encoded string.
// Format byte 0x01 = Latin-1, 0x00 = UTF-16 LE.
func decodeChromiumString(b []byte) (string, error) {
	if len(b) == 0 {
		return "", fmt.Errorf("empty chromium string")
	}
	switch b[0] {
	case chromiumStringLatin1Format:
		return string(b[1:]), nil
	case chromiumStringUTF16Format:
		return decodeUTF16LE(b[1:])
	default:
		return "", fmt.Errorf("unknown chromium string format 0x%02x", b[0])
	}
}

// decodeUTF16LE decodes a UTF-16 Little-Endian byte slice to a Go string.
func decodeUTF16LE(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	if len(b)%2 != 0 {
		return "", fmt.Errorf("invalid UTF-16 byte length %d", len(b))
	}
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16s)), nil
}

func decodeLocalStorageValue(value []byte) string {
	if len(value) >= maxLocalStorageValueLength {
		return fmt.Sprintf(
			"value is too long, length is %d, supported max length is %d",
			len(value), maxLocalStorageValueLength,
		)
	}
	decoded, err := decodeChromiumString(value)
	if err != nil {
		return fmt.Sprintf("unsupported value encoding: %v", err)
	}
	return decoded
}

func extractSessionStorage(path string) ([]types.StorageEntry, error) {
	return extractLevelDB(path, []byte("-"))
}

// extractLevelDB iterates over all entries in a LevelDB directory,
// splitting each key by the separator into (url, name).
func extractLevelDB(path string, separator []byte) ([]types.StorageEntry, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("leveldb path not found: %s", path)
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var entries []types.StorageEntry
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		url, name := parseStorageKey(iter.Key(), separator)
		if url == "" {
			continue
		}
		entries = append(entries, types.StorageEntry{
			URL:   url,
			Key:   name,
			Value: string(iter.Value()),
		})
	}
	return entries, iter.Error()
}

// parseStorageKey splits a LevelDB key into (url, name) by the given separator.
func parseStorageKey(key, separator []byte) (url, name string) {
	parts := bytes.SplitN(key, separator, 2)
	if len(parts) != 2 {
		return "", ""
	}
	return string(parts[0]), string(parts[1])
}
