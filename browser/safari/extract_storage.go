package safari

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf16"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Modern WebKit (Safari 17+) stores localStorage under a nested, partitioned layout rooted at
// either WebsiteDataStore/<uuid>/Origins (per named profile) or WebsiteData/Default
// (the pre-profile default store). Within that root:
//
//	<root>/<top-frame-hash>/<frame-hash>/origin                         — binary; encodes top+frame origins
//	<root>/<top-frame-hash>/<frame-hash>/LocalStorage/localstorage.sqlite3
//
// top-hash == frame-hash ⇒ first-party; they differ for third-party partitioned storage.
// We report the frame origin because that's what window.localStorage exposes to JS.
// ItemTable: (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL);
// value BLOBs are UTF-16 LE strings.
//
// The flat "LocalStorage/<scheme>_<host>_<port>.localstorage" directory that older builds used
// is empty on current Safari and is no longer a supported source.

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

// Port marker values after the (scheme, host) pair in an origin block.
// 0x00  → port is the scheme default (stored as 0).
// 0x01  → next two bytes are a uint16_le port.
const (
	originPortDefaultMarker = 0x00
	originPortExplicitFlag  = 0x01
)

func extractLocalStorage(root string) ([]types.StorageEntry, error) {
	dirs, err := findOriginDataDirs(root)
	if err != nil {
		return nil, err
	}

	var entries []types.StorageEntry
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
			entries = append(entries, types.StorageEntry{
				URL:   origin,
				Key:   it.key,
				Value: it.value,
			})
		}
	}
	return entries, nil
}

// countLocalStorage sums ItemTable row counts across every origin DB under root without
// parsing origin files or decoding values — CountEntries callers only need the total, not the
// URLs or plaintext. COUNT(key) naturally excludes NULL keys, matching the same skip rule
// applied by readLocalStorageFile, so count and extract stay in sync.
func countLocalStorage(root string) (int, error) {
	dirs, err := findOriginDataDirs(root)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, od := range dirs {
		dbPath := filepath.Join(od, webkitLocalStorageSubdir, webkitLocalStorageDB)
		n, err := countLocalStorageFile(dbPath)
		if err != nil {
			log.Debugf("safari localstorage: count %s: %v", dbPath, err)
			continue
		}
		total += n
	}
	return total, nil
}

func countLocalStorageFile(path string) (int, error) {
	// mode=ro (no immutable) so SQLite replays the copied -wal sidecar — this surfaces entries
	// Safari has committed to WAL but not yet checkpointed to the main DB. Writes SQLite might
	// make to the temp-copy's -shm during replay are harmless; the Session cleanup removes
	// everything. Live-file reads (profiles.go) still use immutable=1 to stay off the real WAL.
	dsn := "file:" + path + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", path, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return 0, fmt.Errorf("ping %s: %w", path, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(key) FROM ItemTable`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count ItemTable: %w", err)
	}
	return count, nil
}

// findOriginDataDirs returns <root>/<h1>/<h2>/ paths that contain both an "origin" file and
// a "LocalStorage/localstorage.sqlite3" database. Non-directory entries, the "salt" sibling,
// and partition dirs without localStorage data are silently skipped.
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

// originEndpoint is one half of an origin file (top-frame or frame). Port 0 means the scheme
// default (443 for https, 80 for http) and is omitted from the URL rendering.
type originEndpoint struct {
	scheme string
	host   string
	port   uint16
}

// readOriginFile parses WebKit's SecurityOrigin binary serialization and returns the frame
// origin URL (scheme://host[:port]). The file holds two origin blocks back-to-back: top-frame
// then frame. When the frame block is missing/unreadable we fall back to the top-frame so we
// can still attribute the data to *something* meaningful.
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
		// Partitioned info unavailable — attribute to the top-frame origin.
		frame = top
	}
	if frame.scheme == "" || frame.host == "" {
		return "", fmt.Errorf("origin file missing scheme/host")
	}
	return formatOriginURL(frame), nil
}

// readOriginBlock reads one origin block: scheme record, host record, port marker.
// Returns the parsed endpoint and the byte offset immediately after the block.
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

// decodeLatin1 converts ISO-8859-1 bytes to a valid UTF-8 Go string. Latin-1 byte values map
// 1:1 to Unicode code points U+0000–U+00FF. Mirrors the helper in chromium/extract_storage.go.
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
	// mode=ro (no immutable) — see countLocalStorageFile for the WAL-replay rationale; the same
	// live-vs-temp split applies here. ORDER BY key, rowid makes exports byte-for-byte stable
	// across runs and SQLite versions.
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
			// NULL keys would collide with legitimate empty-string keys in the output and are
			// not meaningful localStorage entries. The UNIQUE constraint in ItemTable still
			// permits multiple NULL rows in SQLite, so we filter them here.
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

// decodeLocalStorageValue treats the BLOB as UTF-16 LE. Values at or above the cap are replaced
// with a size marker to keep JSON/CSV output bounded, matching chromium/extract_storage.go.
func decodeLocalStorageValue(b []byte) string {
	if len(b) >= maxLocalStorageValueLength {
		return fmt.Sprintf(
			"value is too long, length is %d, supported max length is %d",
			len(b), maxLocalStorageValueLength,
		)
	}
	return decodeUTF16LE(b)
}

// decodeUTF16LE returns the input as a Go string on odd-length (malformed) inputs; WebKit values
// are always even-length in practice but we don't want a stray byte to drop a whole row.
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
