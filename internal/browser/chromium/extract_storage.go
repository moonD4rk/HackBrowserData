package chromium

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// Key prefixes and format bytes from Chromium's components/services/storage/dom_storage/local_storage_impl.cc.
const (
	localStorageVersionKey     = "VERSION"
	localStorageMetaPrefix     = "META:"
	localStorageMetaAccessKey  = "METAACCESS:"
	localStorageDataPrefix     = '_'
	chromiumStringUTF16Format  = 0
	chromiumStringLatin1Format = 1
)

const maxLocalStorageValueLength = 2048

// readOnly keeps the open from replaying the journal into new files in the copy, a write that can fail on Windows.
var readOnly = &opt.Options{ReadOnly: true}

func extractLocalStorage(path string, out browser.Collector) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("leveldb path %q: %w", path, err)
	}
	database, err := leveldb.OpenFile(path, readOnly)
	if err != nil {
		return err
	}
	defer database.Close()

	var entries []record.LocalStorage
	cursor := database.NewIterator(nil, nil)
	defer cursor.Release()

	for cursor.Next() {
		entry, ok := parseLocalStorageEntry(cursor.Key(), cursor.Value())
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	browser.AddAll(out, entries)
	return cursor.Error()
}

// parseLocalStorageEntry drops VERSION and unknown keys and keeps META entries with IsMeta set.
func parseLocalStorageEntry(key, value []byte) (record.LocalStorage, bool) {
	switch {
	case bytes.Equal(key, []byte(localStorageVersionKey)):
		return record.LocalStorage{}, false
	case bytes.HasPrefix(key, []byte(localStorageMetaAccessKey)):
		return record.LocalStorage{
			IsMeta: true,
			URL:    string(bytes.TrimPrefix(key, []byte(localStorageMetaAccessKey))),
			Value:  fmt.Sprintf("meta data, value bytes is %v", value),
		}, true
	case bytes.HasPrefix(key, []byte(localStorageMetaPrefix)):
		return record.LocalStorage{
			IsMeta: true,
			URL:    string(bytes.TrimPrefix(key, []byte(localStorageMetaPrefix))),
			Value:  fmt.Sprintf("meta data, value bytes is %v", value),
		}, true
	case len(key) > 0 && key[0] == localStorageDataPrefix:
		return parseLocalStorageDataEntry(key[1:], value), true
	default:
		return record.LocalStorage{}, false
	}
}

// parseLocalStorageDataEntry decodes a data entry with format: origin\x00<encoded-key>.
func parseLocalStorageDataEntry(key, value []byte) record.LocalStorage {
	entry := record.LocalStorage{
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

// decodeChromiumString switches on the first byte: 0x01 is Latin-1, 0x00 is UTF-16 LE.
func decodeChromiumString(encoded []byte) (string, error) {
	if len(encoded) == 0 {
		return "", fmt.Errorf("empty chromium string")
	}
	switch encoded[0] {
	case chromiumStringLatin1Format:
		return decodeLatin1(encoded[1:]), nil
	case chromiumStringUTF16Format:
		return decodeUTF16LE(encoded[1:])
	default:
		return "", fmt.Errorf("unknown chromium string format 0x%02x", encoded[0])
	}
}

func decodeLatin1(encoded []byte) string {
	runes := make([]rune, len(encoded))
	for i, char := range encoded {
		runes[i] = rune(char)
	}
	return string(runes)
}

func decodeUTF16LE(encoded []byte) (string, error) {
	if len(encoded) == 0 {
		return "", nil
	}
	if len(encoded)%2 != 0 {
		return "", fmt.Errorf("invalid UTF-16 byte length %d", len(encoded))
	}
	units := make([]uint16, len(encoded)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(encoded[i*2:])
	}
	return string(utf16.Decode(units)), nil
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

// extractSessionStorage joins namespace-<guid>-<origin> -> map_id with map-<map_id>-<key> -> UTF-16 LE value.
func extractSessionStorage(path string, out browser.Collector) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("leveldb path %q: %w", path, err)
	}
	database, err := leveldb.OpenFile(path, readOnly)
	if err != nil {
		return err
	}
	defer database.Close()

	originByMapID := make(map[string]string)
	namespaceCursor := database.NewIterator(nil, nil)
	for namespaceCursor.Next() {
		key := string(namespaceCursor.Key())
		if !strings.HasPrefix(key, "namespace-") {
			continue
		}
		// Namespace GUIDs use underscores, so the first -https://, -http://, or -chrome:// starts the origin.
		origin := extractNamespaceOrigin(key)
		if origin == "" {
			continue
		}
		mapID := string(namespaceCursor.Value())
		originByMapID[mapID] = origin
	}
	namespaceCursor.Release()
	if err := namespaceCursor.Error(); err != nil {
		return fmt.Errorf("read namespace entries: %w", err)
	}

	var entries []record.SessionStorage
	mapCursor := database.NewIterator(nil, nil)
	defer mapCursor.Release()

	mapPrefix := []byte("map-")
	for mapCursor.Next() {
		key := mapCursor.Key()
		if !bytes.HasPrefix(key, mapPrefix) {
			continue
		}
		rest := key[len(mapPrefix):] // "<map_id>-<key_name>"
		separatorIndex := bytes.IndexByte(rest, '-')
		if separatorIndex < 0 {
			continue
		}
		mapID := string(rest[:separatorIndex])
		keyName := string(rest[separatorIndex+1:])

		origin := originByMapID[mapID]
		if origin == "" {
			origin = mapID // fallback to map_id if namespace not found
		}

		value := decodeSessionStorageValue(mapCursor.Value())
		entries = append(entries, record.SessionStorage{
			URL:   origin,
			Key:   keyName,
			Value: value,
		})
	}
	browser.AddAll(out, entries)
	return mapCursor.Error()
}

func extractNamespaceOrigin(key string) string {
	for _, prefix := range []string{"-https://", "-http://", "-chrome://"} {
		index := strings.Index(key, prefix)
		if index >= 0 {
			return key[index+1:]
		}
	}
	return ""
}

// decodeSessionStorageValue reads raw UTF-16 LE, which has no localStorage format byte.
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
