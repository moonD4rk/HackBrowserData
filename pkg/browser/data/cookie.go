package data

import (
	"database/sql"
	"fmt"
	"sort"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/pkg/decrypter"
	"hack-browser-data/utils"

	_ "github.com/mattn/go-sqlite3"
)

type ChromiumCookie []cookie

func (c *ChromiumCookie) Parse(masterKey []byte) error {
	cookieDB, err := sql.Open("sqlite3", consts.ChromiumCookieFilename)
	if err != nil {
		return err
	}
	defer cookieDB.Close()
	rows, err := cookieDB.Query(queryChromiumCookie)
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
			fmt.Println(err)
		}

		cookie := cookie{
			KeyName:      key,
			Host:         host,
			Path:         path,
			encryptValue: encryptValue,
			IsSecure:     utils.IntToBool(isSecure),
			IsHTTPOnly:   utils.IntToBool(isHTTPOnly),
			HasExpire:    utils.IntToBool(hasExpire),
			IsPersistent: utils.IntToBool(isPersistent),
			CreateDate:   utils.TimeEpochFormat(createDate),
			ExpireDate:   utils.TimeEpochFormat(expireDate),
		}
		// TODO: replace DPAPI
		if len(encryptValue) > 0 {
			if masterKey == nil {
				value, err = decrypter.DPApi(encryptValue)
				if err != nil {
					fmt.Println(err)
				}
			} else {
				value, err = decrypter.ChromePass(masterKey, encryptValue)
				if err != nil {
					fmt.Println(err)
				}
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

type FirefoxCookie []cookie

func (f *FirefoxCookie) Parse(masterKey []byte) error {
	cookieDB, err := sql.Open("sqlite3", consts.FirefoxCookieFilename)
	if err != nil {
		return err
	}
	defer cookieDB.Close()
	rows, err := cookieDB.Query(queryFirefoxCookie)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			name, value, host, path string
			isSecure, isHttpOnly    int
			creationTime, expiry    int64
		)
		if err = rows.Scan(&name, &value, &host, &path, &creationTime, &expiry, &isSecure, &isHttpOnly); err != nil {
			fmt.Println(err)
		}
		*f = append(*f, cookie{
			KeyName:    name,
			Host:       host,
			Path:       path,
			IsSecure:   utils.IntToBool(isSecure),
			IsHTTPOnly: utils.IntToBool(isHttpOnly),
			CreateDate: utils.TimeStampFormat(creationTime / 1000000),
			ExpireDate: utils.TimeStampFormat(expiry),
			Value:      value,
		})
	}
	return nil
}

func (f *FirefoxCookie) Name() string {
	return "cookie"
}
