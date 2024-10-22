package sessionstorage

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
	extractor.RegisterExtractor(types.ChromiumSessionStorage, func() extractor.Extractor {
		return new(ChromiumSessionStorage)
	})
	extractor.RegisterExtractor(types.FirefoxSessionStorage, func() extractor.Extractor {
		return new(FirefoxSessionStorage)
	})
}

type ChromiumSessionStorage []session

type session struct {
	IsMeta bool
	URL    string
	Key    string
	Value  string
}

const maxLocalStorageValueLength = 1024 * 2

func (c *ChromiumSessionStorage) Extract(_ []byte) error {
	db, err := leveldb.OpenFile(types.ChromiumSessionStorage.TempFilename(), nil)
	if err != nil {
		return err
	}
	defer os.RemoveAll(types.ChromiumSessionStorage.TempFilename())
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		s := new(session)
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

func (c *ChromiumSessionStorage) Name() string {
	return "sessionStorage"
}

func (c *ChromiumSessionStorage) Len() int {
	return len(*c)
}

func (s *session) fillKey(b []byte) {
	keys := bytes.Split(b, []byte("-"))
	if len(keys) == 1 && bytes.HasPrefix(keys[0], []byte("META:")) {
		s.IsMeta = true
		s.fillMetaHeader(keys[0])
	}
	if len(keys) == 2 && bytes.HasPrefix(keys[0], []byte("_")) {
		s.fillHeader(keys[0], keys[1])
	}
	if len(keys) == 3 {
		if string(keys[0]) == "map" {
			s.Key = string(keys[2])
		} else if string(keys[0]) == "namespace" {
			s.URL = string(keys[2])
			s.Key = string(keys[1])
		}
	}
}

func (s *session) fillMetaHeader(b []byte) {
	s.URL = string(bytes.Trim(b, "META:"))
}

func (s *session) fillHeader(url, key []byte) {
	s.URL = string(bytes.Trim(url, "_"))
	s.Key = string(bytes.Trim(key, "\x01"))
}

func convertUTF16toUTF8(source []byte, endian unicode.Endianness) ([]byte, error) {
	r, _, err := transform.Bytes(unicode.UTF16(endian, unicode.IgnoreBOM).NewDecoder(), source)
	return r, err
}

// fillValue fills value of the storage
// TODO: support unicode charter
func (s *session) fillValue(b []byte) {
	value := bytes.Map(byteutil.OnSplitUTF8Func, b)
	s.Value = string(value)
}

type FirefoxSessionStorage []session

const (
	querySessionStorage = `SELECT originKey, key, value FROM webappsstore2`
	closeJournalMode    = `PRAGMA journal_mode=off`
)

func (f *FirefoxSessionStorage) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.FirefoxSessionStorage.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.FirefoxSessionStorage.TempFilename())
	defer db.Close()

	_, err = db.Exec(closeJournalMode)
	if err != nil {
		log.Errorf("close journal mode error: %v", err)
	}
	rows, err := db.Query(querySessionStorage)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var originKey, key, value string
		if err = rows.Scan(&originKey, &key, &value); err != nil {
			log.Errorf("scan session storage error: %v", err)
		}
		s := new(session)
		s.fillFirefox(originKey, key, value)
		*f = append(*f, *s)
	}
	return nil
}

func (s *session) fillFirefox(originKey, key, value string) {
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

func (f *FirefoxSessionStorage) Name() string {
	return "sessionStorage"
}

func (f *FirefoxSessionStorage) Len() int {
	return len(*f)
}
