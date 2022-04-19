package localstorage

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils/typeutil"
)

type ChromiumLocalStorage []storage

type storage struct {
	IsMeta bool
	URL    string
	Key    string
	Value  string
}

func (c *ChromiumLocalStorage) Parse(masterKey []byte) error {
	db, err := leveldb.OpenFile(item.TempChromiumLocalStorage, nil)
	if err != nil {
		return err
	}
	defer os.RemoveAll(item.TempChromiumLocalStorage)
	// log.Info("parsing local storage now")
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		// don't parse value upper than 5kB
		if len(value) > 1024*5 {
			continue
		}
		var s = new(storage)
		s.fillKey(key)
		s.fillValue(value)
		// don't save meta data
		if s.IsMeta {
			continue
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

// fillValue fills value of the storage
// TODO: support unicode charter
func (s *storage) fillValue(b []byte) {
	t := fmt.Sprintf("%c", b)
	m := strings.NewReplacer(" ", "", "\x00", "", "\x01", "").Replace(t)
	s.Value = m
}

type FirefoxLocalStorage []storage

const (
	queryFirefoxHistory = `SELECT originKey, key, value FROM webappsstore2`
	closeJournalMode    = `PRAGMA journal_mode=off`
)

func (f *FirefoxLocalStorage) Parse(masterKey []byte) error {
	db, err := sql.Open("sqlite3", item.TempFirefoxLocalStorage)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	defer os.Remove(item.TempFirefoxLocalStorage)
	defer db.Close()
	_, err = db.Exec(closeJournalMode)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(queryFirefoxHistory)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			originKey, key, value string
		)
		if err = rows.Scan(&originKey, &key, &value); err != nil {
			log.Warn(err)
		}
		var s = new(storage)
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
