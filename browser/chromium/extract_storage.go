package chromium

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
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
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("leveldb path %q: %w", path, err)
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
// Returns false for VERSION entries and any unrecognized keys. META entries are kept with IsMeta=true.
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
		return decodeLatin1(b[1:]), nil
	case chromiumStringUTF16Format:
		return decodeUTF16LE(b[1:])
	default:
		return "", fmt.Errorf("unknown chromium string format 0x%02x", b[0])
	}
}

// decodeLatin1 converts ISO-8859-1 bytes to a valid UTF-8 Go string.
// Latin-1 byte values map 1:1 to Unicode code points U+0000–U+00FF.
func decodeLatin1(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
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

// extractSessionStorage reads Chromium session storage LevelDB.
//
// LevelDB key format:
//
//	namespace-<guid>-<origin> → <map_id>   (origin mapping)
//	map-<map_id>-<key_name>  → <value>     (actual data, UTF-16 LE)
//	next-map-id / version                  (metadata, skipped)
func extractSessionStorage(path string) ([]types.StorageEntry, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("leveldb path %q: %w", path, err)
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Pass 1: build map_id → origin lookup from namespace entries.
	// Key: "namespace-<guid>-<origin>", Value: "<map_id>" (ASCII digits).
	originByMapID := make(map[string]string)
	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := string(iter.Key())
		if !strings.HasPrefix(key, "namespace-") {
			continue
		}
		// Extract origin by finding "-https://", "-http://", or "-chrome://" in the key.
		// Namespace GUIDs use underscores (e.g., "03b2df3a_0d95_4d55_ae57_...") so
		// there is no ambiguity with the origin separator.
		origin := extractNamespaceOrigin(key)
		if origin == "" {
			continue
		}
		mapID := string(iter.Value())
		originByMapID[mapID] = origin
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("read namespace entries: %w", err)
	}

	// Pass 2: read map entries and resolve origins.
	var entries []types.StorageEntry
	iter2 := db.NewIterator(nil, nil)
	defer iter2.Release()

	mapPrefix := []byte("map-")
	for iter2.Next() {
		key := iter2.Key()
		if !bytes.HasPrefix(key, mapPrefix) {
			continue
		}
		rest := key[len(mapPrefix):] // "<map_id>-<key_name>"
		sep := bytes.IndexByte(rest, '-')
		if sep < 0 {
			continue
		}
		mapID := string(rest[:sep])
		keyName := string(rest[sep+1:])

		origin := originByMapID[mapID]
		if origin == "" {
			origin = mapID // fallback to map_id if namespace not found
		}

		value := decodeSessionStorageValue(iter2.Value())
		entries = append(entries, types.StorageEntry{
			URL:   origin,
			Key:   keyName,
			Value: value,
		})
	}
	return entries, iter2.Error()
}

// extractNamespaceOrigin extracts the origin from a namespace key.
// Key format: "namespace-<guid_with_underscores>-<origin>"
// The GUID uses underscores, so we find the origin by looking for "-http" or "-chrome".
func extractNamespaceOrigin(key string) string {
	for _, prefix := range []string{"-https://", "-http://", "-chrome://"} {
		idx := strings.Index(key, prefix)
		if idx >= 0 {
			return key[idx+1:]
		}
	}
	return ""
}

// decodeSessionStorageValue decodes a session storage value.
// Values are raw UTF-16 LE (no format byte prefix, unlike localStorage).
func decodeSessionStorageValue(value []byte) string {
	if len(value) == 0 {
		return ""
	}
	if len(value)%2 == 0 {
		decoded, err := decodeUTF16LE(value)
		if err == nil {
			return decoded
		}
	}
	return string(value)
}
