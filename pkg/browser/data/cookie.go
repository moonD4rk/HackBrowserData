package data

import (
	"database/sql"
	"fmt"
	"sort"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/pkg/decrypter"
	"hack-browser-data/utils"
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
