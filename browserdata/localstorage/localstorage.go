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
	"github.com/moond4rk/hackbrowserdata/utils/byteutil"
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

func (c *ChromiumLocalStorage) Extract(_ []byte) error {
	db, err := leveldb.OpenFile(types.ChromiumLocalStorage.TempFilename(), nil)
	if err != nil {
		return err
	}
	defer os.RemoveAll(types.ChromiumLocalStorage.TempFilename())
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		s := new(storage)
		s.fillKey(key)
		// don't all value upper than 2KB
		if len(value) < maxLocalStorageValueLength {
			s.fillValue(value)
		} else {
			s.Value = fmt.Sprintf("value is too long, length is %d, supported max length is %d", len(value), maxLocalStorageValueLength)
		}
		if s.IsMeta {
			s.Value = fmt.Sprintf("meta data, value bytes is %v", value)
		}
		*c = append(*c, *s)
	}
	iter.Release()
	err = iter.Error()
	return err
}

func (c *ChromiumLocalStorage) Name() string {
	return "localStorage"
}

func (c *ChromiumLocalStorage) Len() int {
	return len(*c)
}

func (s *storage) fillKey(b []byte) {
	keys := bytes.Split(b, []byte("\x00"))
	if len(keys) == 1 && bytes.HasPrefix(keys[0], []byte("META:")) {
		s.IsMeta = true
		s.fillMetaHeader(keys[0])
	}
	if len(keys) == 2 && bytes.HasPrefix(keys[0], []byte("_")) {
		s.fillHeader(keys[0], keys[1])
	}
}

func (s *storage) fillMetaHeader(b []byte) {
	s.URL = string(bytes.Trim(b, "META:"))
}

func (s *storage) fillHeader(url, key []byte) {
	s.URL = string(bytes.Trim(url, "_"))
	s.Key = string(bytes.Trim(key, "\x01"))
}

func convertUTF16toUTF8(source []byte, endian unicode.Endianness) ([]byte, error) {
	r, _, err := transform.Bytes(unicode.UTF16(endian, unicode.IgnoreBOM).NewDecoder(), source)
	return r, err
}

// fillValue fills value of the storage
// TODO: support unicode charter
func (s *storage) fillValue(b []byte) {
	value := bytes.Map(byteutil.OnSplitUTF8Func, b)
	s.Value = string(value)
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
		log.Errorf("close journal mode error: %v", err)
	}
	rows, err := db.Query(queryLocalStorage)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var originKey, key, value string
		if err = rows.Scan(&originKey, &key, &value); err != nil {
			log.Errorf("scan firefox local storage error: %v", err)
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
