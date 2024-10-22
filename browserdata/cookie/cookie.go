package cookie

import (
	"database/sql"
	"os"
	"sort"
	"time"

	// import sqlite3 driver
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/extractor"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

func init() {
	extractor.RegisterExtractor(types.ChromiumCookie, func() extractor.Extractor {
		return new(ChromiumCookie)
	})
	extractor.RegisterExtractor(types.FirefoxCookie, func() extractor.Extractor {
		return new(FirefoxCookie)
	})
}

type ChromiumCookie []cookie

type cookie struct {
	Host         string
	Path         string
	KeyName      string
	encryptValue []byte
	Value        string
	IsSecure     bool
	IsHTTPOnly   bool
	HasExpire    bool
	IsPersistent bool
	CreateDate   time.Time
	ExpireDate   time.Time
}

const (
	queryChromiumCookie = `SELECT name, encrypted_value, host_key, path, creation_utc, expires_utc, is_secure, is_httponly, has_expires, is_persistent FROM cookies`
)

func (c *ChromiumCookie) Extract(masterKey []byte) error {
	db, err := sql.Open("sqlite", types.ChromiumCookie.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.ChromiumCookie.TempFilename())
	defer db.Close()
	rows, err := db.Query(queryChromiumCookie)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			key, host, path                               string
			isSecure, isHTTPOnly, hasExpire, isPersistent int
			createDate, expireDate                        int64
			value, encryptValue                           []byte
		)
		if err = rows.Scan(&key, &encryptValue, &host, &path, &createDate, &expireDate, &isSecure, &isHTTPOnly, &hasExpire, &isPersistent); err != nil {
			log.Errorf("scan chromium cookie error: %v", err)
		}

		cookie := cookie{
			KeyName:      key,
			Host:         host,
			Path:         path,
			encryptValue: encryptValue,
			IsSecure:     typeutil.IntToBool(isSecure),
			IsHTTPOnly:   typeutil.IntToBool(isHTTPOnly),
			HasExpire:    typeutil.IntToBool(hasExpire),
			IsPersistent: typeutil.IntToBool(isPersistent),
			CreateDate:   typeutil.TimeEpoch(createDate),
			ExpireDate:   typeutil.TimeEpoch(expireDate),
		}
		if len(encryptValue) > 0 {
			if len(masterKey) == 0 {
				value, err = crypto.DecryptWithDPAPI(encryptValue)
			} else {
				value, err = crypto.DecryptWithChromium(masterKey, encryptValue)
			}
			if err != nil {
				log.Errorf("decrypt chromium cookie error: %v", err)
			}
		}
		cookie.Value = string(value)
		*c = append(*c, cookie)
	}
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].CreateDate.After((*c)[j].CreateDate)
	})
	return nil
}

func (c *ChromiumCookie) Name() string {
	return "cookie"
}

func (c *ChromiumCookie) Len() int {
	return len(*c)
}

type FirefoxCookie []cookie

const (
	queryFirefoxCookie = `SELECT name, value, host, path, creationTime, expiry, isSecure, isHttpOnly FROM moz_cookies`
)

func (f *FirefoxCookie) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.FirefoxCookie.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.FirefoxCookie.TempFilename())
	defer db.Close()

	rows, err := db.Query(queryFirefoxCookie)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			name, value, host, path string
			isSecure, isHTTPOnly    int
			creationTime, expiry    int64
		)
		if err = rows.Scan(&name, &value, &host, &path, &creationTime, &expiry, &isSecure, &isHTTPOnly); err != nil {
			log.Errorf("scan firefox cookie error: %v", err)
		}
		*f = append(*f, cookie{
			KeyName:    name,
			Host:       host,
			Path:       path,
			IsSecure:   typeutil.IntToBool(isSecure),
			IsHTTPOnly: typeutil.IntToBool(isHTTPOnly),
			CreateDate: typeutil.TimeStamp(creationTime / 1000000),
			ExpireDate: typeutil.TimeStamp(expiry),
			Value:      value,
		})
	}

	sort.Slice(*f, func(i, j int) bool {
		return (*f)[i].CreateDate.After((*f)[j].CreateDate)
	})
	return nil
}

func (f *FirefoxCookie) Name() string {
	return "cookie"
}

func (f *FirefoxCookie) Len() int {
	return len(*f)
}
