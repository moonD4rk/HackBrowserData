package safari

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf16"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// Safari 17+ keeps localStorage at <root>/<top-hash>/<frame-hash>/{origin,LocalStorage/localstorage.sqlite3},
// where root is WebsiteDataStore/<uuid>/Origins or WebsiteData/Default; the hashes differ for third-party storage.
// The frame origin is reported, since window.localStorage exposes it; the old flat layout is no longer read.

const (
	webkitOriginFile         = "origin"
	webkitLocalStorageSubdir = "LocalStorage"
	webkitLocalStorageDB     = "localstorage.sqlite3"
	webkitOriginSaltName     = "salt" // HMAC salt sibling of the <hash> dirs; not a data dir

	maxLocalStorageValueLength = 2048
)

// origin file encoding-byte constants (WebCore SecurityOrigin serialization).
const (
	originEncASCII = 0x01 // Latin-1 / ASCII
	originEncUTF16 = 0x00 // UTF-16 LE
)

// After scheme and host, 0x00 means the default port and 0x01 a uint16 LE port in the next two bytes.
const (
	originPortDefaultMarker = 0x00
	originPortExplicitFlag  = 0x01
)

func extractLocalStorage(root string, out browser.Collector) error {
	dirs, err := findOriginDataDirs(root)
	if err != nil {
		return err
	}

	var entries []record.LocalStorage
	for _, originDir := range dirs {
		origin, err := readOriginFile(filepath.Join(originDir, webkitOriginFile))
		if err != nil {
			log.Debugf("safari localstorage: origin %s: %v", originDir, err)
			continue
		}
		dbPath := filepath.Join(originDir, webkitLocalStorageSubdir, webkitLocalStorageDB)
		items, err := readLocalStorageFile(dbPath)
		if err != nil {
			log.Debugf("safari localstorage: db %s: %v", dbPath, err)
			continue
		}
		for _, item := range items {
			entries = append(entries, record.LocalStorage{
				URL:   origin,
				Key:   item.key,
				Value: item.value,
			})
		}
	}
	emit(out, entries)
	return nil
}

// findOriginDataDirs counts only dirs with both an origin file and a localstorage.sqlite3; the salt sibling is skipped.
func findOriginDataDirs(root string) ([]string, error) {
	topEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read origins root %s: %w", root, err)
	}
	var out []string
	for _, top := range topEntries {
		if !top.IsDir() || top.Name() == webkitOriginSaltName {
			continue
		}
		topPath := filepath.Join(root, top.Name())
		frameEntries, err := os.ReadDir(topPath)
		if err != nil {
			continue
		}
		for _, frame := range frameEntries {
			if !frame.IsDir() {
				continue
			}
			framePath := filepath.Join(topPath, frame.Name())
			if _, err := os.Stat(filepath.Join(framePath, webkitOriginFile)); err != nil {
				continue
			}
			dbPath := filepath.Join(framePath, webkitLocalStorageSubdir, webkitLocalStorageDB)
			if _, err := os.Stat(dbPath); err != nil {
				continue
			}
			out = append(out, framePath)
		}
	}
	return out, nil
}

// originEndpoint leaves port 0, the scheme's default, out of the URL.
type originEndpoint struct {
	scheme string
	host   string
	port   uint16
}

// readOriginFile reads the top-frame block, then the frame block; without a readable frame the top frame is reported.
func readOriginFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read origin file %s: %w", path, err)
	}
	top, position, topErr := readOriginBlock(data, 0)
	if topErr != nil {
		return "", fmt.Errorf("parse top-frame origin: %w", topErr)
	}
	frame, _, frameErr := readOriginBlock(data, position)
	if frameErr != nil {
		frame = top
	}
	if frame.scheme == "" || frame.host == "" {
		return "", fmt.Errorf("origin file missing scheme/host")
	}
	return formatOriginURL(frame), nil
}

func readOriginBlock(data []byte, position int) (originEndpoint, int, error) {
	var endpoint originEndpoint
	var err error
	endpoint.scheme, position, err = readOriginString(data, position)
	if err != nil {
		return endpoint, position, err
	}
	endpoint.host, position, err = readOriginString(data, position)
	if err != nil {
		return endpoint, position, err
	}
	if position >= len(data) {
		return endpoint, position, fmt.Errorf("unexpected EOF before port marker")
	}
	marker := data[position]
	position++
	switch marker {
	case originPortDefaultMarker:
		endpoint.port = 0
	case originPortExplicitFlag:
		if position+2 > len(data) {
			return endpoint, position, fmt.Errorf("truncated port value at offset %d", position)
		}
		endpoint.port = binary.LittleEndian.Uint16(data[position : position+2])
		position += 2
	default:
		return endpoint, position, fmt.Errorf("unexpected port marker 0x%02x at offset %d", marker, position-1)
	}
	return endpoint, position, nil
}

// readOriginString consumes one length-prefixed record (uint32_le length + encoding byte + data).
func readOriginString(data []byte, position int) (string, int, error) {
	if position+5 > len(data) {
		return "", position, fmt.Errorf("truncated string record at offset %d", position)
	}
	length := int(binary.LittleEndian.Uint32(data[position : position+4]))
	encoding := data[position+4]
	position += 5
	if length < 0 || position+length > len(data) {
		return "", position, fmt.Errorf("string record overruns buffer: length %d at offset %d", length, position-5)
	}
	chunk := data[position : position+length]
	position += length
	switch encoding {
	case originEncASCII:
		return decodeLatin1(chunk), position, nil
	case originEncUTF16:
		return decodeUTF16LE(chunk), position, nil
	default:
		return decodeLatin1(chunk), position, nil
	}
}

func decodeLatin1(encoded []byte) string {
	runes := make([]rune, len(encoded))
	for i, char := range encoded {
		runes[i] = rune(char)
	}
	return string(runes)
}

func formatOriginURL(endpoint originEndpoint) string {
	url := endpoint.scheme + "://" + endpoint.host
	if endpoint.port != 0 {
		url += fmt.Sprintf(":%d", endpoint.port)
	}
	return url
}

type localStorageItem struct {
	key   string
	value string
}

func readLocalStorageFile(path string) ([]localStorageItem, error) {
	// mode=ro without immutable lets SQLite replay the copied -wal, so entries Safari has not checkpointed appear.
	// ORDER BY key, rowid keeps exports stable across runs and SQLite versions.
	dsn := "file:" + path + "?mode=ro"
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer database.Close()
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}

	rows, err := database.Query(`SELECT key, value FROM ItemTable ORDER BY key, rowid`)
	if err != nil {
		return nil, fmt.Errorf("query ItemTable: %w", err)
	}
	defer rows.Close()

	var items []localStorageItem
	for rows.Next() {
		var key sql.NullString
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			log.Debugf("safari localstorage: scan row in %s: %v", path, err)
			continue
		}
		if !key.Valid {
			// NULL keys would collide with empty-string keys in the output, and ItemTable's UNIQUE still allows several.
			log.Debugf("safari localstorage: skip row with NULL key in %s", path)
			continue
		}
		items = append(items, localStorageItem{
			key:   key.String,
			value: decodeLocalStorageValue(value),
		})
	}
	return items, rows.Err()
}

// decodeLocalStorageValue turns values at or above the cap into a size marker to bound the output, as Chromium's does.
func decodeLocalStorageValue(encoded []byte) string {
	if len(encoded) >= maxLocalStorageValueLength {
		return fmt.Sprintf(
			"value is too long, length is %d, supported max length is %d",
			len(encoded), maxLocalStorageValueLength,
		)
	}
	return decodeUTF16LE(encoded)
}

// decodeUTF16LE returns an odd-length value as is, so a stray byte does not drop the row.
func decodeUTF16LE(encoded []byte) string {
	if len(encoded) == 0 {
		return ""
	}
	if len(encoded)%2 != 0 {
		return string(encoded)
	}
	units := make([]uint16, len(encoded)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(encoded[i*2:])
	}
	return string(utf16.Decode(units))
}
