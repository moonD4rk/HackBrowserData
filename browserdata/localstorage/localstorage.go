package localstorage

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/moond4rk/hackbrowserdata/extractor"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

func init() {
	extractor.RegisterExtractor(types.ChromiumLocalStorage, func() extractor.Extractor {
		return new(ChromiumLocalStorage)
	})
	extractor.RegisterExtractor(types.FirefoxLocalStorage, func() extractor.Extractor {
		return new(FirefoxLocalStorage)
	})
}

type ChromiumLocalStorage []storage

type storage struct {
	IsMeta bool
	URL    string
	Key    string
	Value  string
}

const maxLocalStorageValueLength = 1024 * 2

const (
	chromiumLocalStorageVersionKey    = "VERSION"
	chromiumLocalStorageMetaPrefix    = "META:"
	chromiumLocalStorageMetaAccessKey = "METAACCESS:"
	chromiumLocalStorageDataPrefix    = '_'
	chromiumStringUTF16Format         = 0
	chromiumStringLatin1Format        = 1
)

func (c *ChromiumLocalStorage) Extract(_ []byte) error {
	entries, err := extractChromiumLocalStorage(types.ChromiumLocalStorage.TempFilename())
	if err != nil {
		return err
	}
	defer os.RemoveAll(types.ChromiumLocalStorage.TempFilename())
	*c = append(*c, entries...)
	return nil
}

func (c *ChromiumLocalStorage) Name() string {
	return "localStorage"
}

func (c *ChromiumLocalStorage) Len() int {
	return len(*c)
}

func extractChromiumLocalStorage(path string) (ChromiumLocalStorage, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var entries ChromiumLocalStorage
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		entry, ok := parseChromiumLocalStorageEntry(iter.Key(), iter.Value())
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, iter.Error()
}

func parseChromiumLocalStorageEntry(key, value []byte) (storage, bool) {
	switch {
	case bytes.Equal(key, []byte(chromiumLocalStorageVersionKey)):
		return storage{}, false
	case bytes.HasPrefix(key, []byte(chromiumLocalStorageMetaAccessKey)):
		return storage{
			IsMeta: true,
			URL:    string(bytes.TrimPrefix(key, []byte(chromiumLocalStorageMetaAccessKey))),
			Value:  fmt.Sprintf("meta data, value bytes is %v", value),
		}, true
	case bytes.HasPrefix(key, []byte(chromiumLocalStorageMetaPrefix)):
		return storage{
			IsMeta: true,
			URL:    string(bytes.TrimPrefix(key, []byte(chromiumLocalStorageMetaPrefix))),
			Value:  fmt.Sprintf("meta data, value bytes is %v", value),
		}, true
	case len(key) > 0 && key[0] == chromiumLocalStorageDataPrefix:
		return parseChromiumLocalStorageDataEntry(key[1:], value), true
	default:
		return storage{}, false
	}
}

func parseChromiumLocalStorageDataEntry(key, value []byte) storage {
	entry := storage{
		Value: decodeChromiumLocalStorageValue(value),
	}

	separator := bytes.IndexByte(key, 0)
	if separator < 0 {
		entry.Key = "unsupported chromium localStorage key encoding: missing origin separator"
		return entry
	}

	entry.URL = string(key[:separator])
	scriptKey, err := decodeChromiumString(key[separator+1:])
	if err != nil {
		entry.Key = fmt.Sprintf("unsupported chromium localStorage key encoding: %v", err)
		return entry
	}
	entry.Key = scriptKey
	return entry
}

func convertUTF16toUTF8(source []byte, endian unicode.Endianness) ([]byte, error) {
	r, _, err := transform.Bytes(unicode.UTF16(endian, unicode.IgnoreBOM).NewDecoder(), source)
	return r, err
}

func decodeChromiumString(b []byte) (string, error) {
	if len(b) == 0 {
		return "", fmt.Errorf("empty chromium string")
	}

	switch b[0] {
	case chromiumStringLatin1Format:
		return string(b[1:]), nil
	case chromiumStringUTF16Format:
		if len(b) == 1 {
			return "", nil
		}
		if (len(b)-1)%2 != 0 {
			return "", fmt.Errorf("invalid UTF-16 byte length %d", len(b)-1)
		}
		value, err := convertUTF16toUTF8(b[1:], unicode.LittleEndian)
		if err != nil {
			return "", err
		}
		return string(value), nil
	default:
		return "", fmt.Errorf("unknown chromium string format 0x%02x", b[0])
	}
}

func decodeChromiumLocalStorageValue(value []byte) string {
	if len(value) >= maxLocalStorageValueLength {
		return fmt.Sprintf(
			"value is too long, length is %d, supported max length is %d",
			len(value),
			maxLocalStorageValueLength,
		)
	}

	decoded, err := decodeChromiumString(value)
	if err != nil {
		return fmt.Sprintf("unsupported chromium localStorage value encoding: %v", err)
	}
	return decoded
}

type FirefoxLocalStorage []storage

const (
	queryLocalStorage = `SELECT originKey, key, value FROM webappsstore2`
	closeJournalMode  = `PRAGMA journal_mode=off`
)

func (f *FirefoxLocalStorage) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.FirefoxLocalStorage.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.FirefoxLocalStorage.TempFilename())
	defer db.Close()

	_, err = db.Exec(closeJournalMode)
	if err != nil {
		log.Debugf("close journal mode error: %v", err)
	}
	rows, err := db.Query(queryLocalStorage)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var originKey, key, value string
		if err = rows.Scan(&originKey, &key, &value); err != nil {
			log.Debugf("scan firefox local storage error: %v", err)
		}
		s := new(storage)
		s.fillFirefox(originKey, key, value)
		*f = append(*f, *s)
	}
	return nil
}

func (s *storage) fillFirefox(originKey, key, value string) {
	// originKey = moc.buhtig.:https:443
	p := strings.Split(originKey, ":")
	h := typeutil.Reverse([]byte(p[0]))
	if bytes.HasPrefix(h, []byte(".")) {
		h = h[1:]
	}
	if len(p) == 3 {
		s.URL = fmt.Sprintf("%s://%s:%s", p[1], string(h), p[2])
	}
	s.Key = key
	s.Value = value
}

func (f *FirefoxLocalStorage) Name() string {
	return "localStorage"
}

func (f *FirefoxLocalStorage) Len() int {
	return len(*f)
}
