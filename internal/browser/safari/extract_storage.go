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
	for _, od := range dirs {
		origin, err := readOriginFile(filepath.Join(od, webkitOriginFile))
		if err != nil {
			log.Debugf("safari localstorage: origin %s: %v", od, err)
			continue
		}
		dbPath := filepath.Join(od, webkitLocalStorageSubdir, webkitLocalStorageDB)
		items, err := readLocalStorageFile(dbPath)
		if err != nil {
			log.Debugf("safari localstorage: db %s: %v", dbPath, err)
			continue
		}
		for _, it := range items {
			entries = append(entries, record.LocalStorage{
				URL:   origin,
				Key:   it.key,
				Value: it.value,
			})
		}
	}
	emit(out, entries)
	return nil
}

// Only dirs holding both an origin file and a localstorage.sqlite3 count; the salt sibling and the rest are skipped.
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

// Port 0 is the scheme's default and is left out of the URL.
type originEndpoint struct {
	scheme string
	host   string
	port   uint16
}

// The file holds the top-frame block, then the frame block; without a readable frame the top frame is reported.
func readOriginFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read origin file %s: %w", path, err)
	}
	top, pos, terr := readOriginBlock(data, 0)
	if terr != nil {
		return "", fmt.Errorf("parse top-frame origin: %w", terr)
	}
	frame, _, ferr := readOriginBlock(data, pos)
	if ferr != nil {
		frame = top
	}
	if frame.scheme == "" || frame.host == "" {
		return "", fmt.Errorf("origin file missing scheme/host")
	}
	return formatOriginURL(frame), nil
}

func readOriginBlock(data []byte, pos int) (originEndpoint, int, error) {
	var ep originEndpoint
	var err error
	ep.scheme, pos, err = readOriginString(data, pos)
	if err != nil {
		return ep, pos, err
	}
	ep.host, pos, err = readOriginString(data, pos)
	if err != nil {
		return ep, pos, err
	}
	if pos >= len(data) {
		return ep, pos, fmt.Errorf("unexpected EOF before port marker")
	}
	marker := data[pos]
	pos++
	switch marker {
	case originPortDefaultMarker:
		ep.port = 0
	case originPortExplicitFlag:
		if pos+2 > len(data) {
			return ep, pos, fmt.Errorf("truncated port value at offset %d", pos)
		}
		ep.port = binary.LittleEndian.Uint16(data[pos : pos+2])
		pos += 2
	default:
		return ep, pos, fmt.Errorf("unexpected port marker 0x%02x at offset %d", marker, pos-1)
	}
	return ep, pos, nil
}

// readOriginString consumes one length-prefixed record (uint32_le length + encoding byte + data).
func readOriginString(data []byte, pos int) (string, int, error) {
	if pos+5 > len(data) {
		return "", pos, fmt.Errorf("truncated string record at offset %d", pos)
	}
	length := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
	enc := data[pos+4]
	pos += 5
	if length < 0 || pos+length > len(data) {
		return "", pos, fmt.Errorf("string record overruns buffer: length %d at offset %d", length, pos-5)
	}
	chunk := data[pos : pos+length]
	pos += length
	switch enc {
	case originEncASCII:
		return decodeLatin1(chunk), pos, nil
	case originEncUTF16:
		return decodeUTF16LE(chunk), pos, nil
	default:
		return decodeLatin1(chunk), pos, nil
	}
}

func decodeLatin1(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

func formatOriginURL(ep originEndpoint) string {
	url := ep.scheme + "://" + ep.host
	if ep.port != 0 {
		url += fmt.Sprintf(":%d", ep.port)
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
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}

	rows, err := db.Query(`SELECT key, value FROM ItemTable ORDER BY key, rowid`)
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

// Values at or above the cap become a size marker to bound the output, as in chromium/extract_storage.go.
func decodeLocalStorageValue(b []byte) string {
	if len(b) >= maxLocalStorageValueLength {
		return fmt.Sprintf(
			"value is too long, length is %d, supported max length is %d",
			len(b), maxLocalStorageValueLength,
		)
	}
	return decodeUTF16LE(b)
}

// An odd-length value comes back as is, so a stray byte does not drop the row.
func decodeUTF16LE(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b)%2 != 0 {
		return string(b)
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16))
}
