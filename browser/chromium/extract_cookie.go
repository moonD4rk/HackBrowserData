package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

const defaultCookieQuery = `SELECT name, encrypted_value, host_key, path,
	creation_utc, expires_utc, is_secure, is_httponly,
	has_expires, is_persistent FROM cookies`

func extractCookies(masterKey []byte, path string) ([]types.CookieEntry, error) {
	cookies, err := sqliteutil.QueryRows(path, false, defaultCookieQuery,
		func(rows *sql.Rows) (types.CookieEntry, error) {
			var (
				name, host, cookiePath  string
				isSecure, isHTTPOnly    int
				hasExpire, isPersistent int
				createdAt, expireAt     int64
				encryptedValue          []byte
			)
			if err := rows.Scan(&name, &encryptedValue, &host, &cookiePath,
				&createdAt, &expireAt, &isSecure, &isHTTPOnly,
				&hasExpire, &isPersistent); err != nil {
				return types.CookieEntry{}, err
			}

			value, _ := decryptValue(masterKey, encryptedValue)
			return types.CookieEntry{
				Name:       name,
				Host:       host,
				Path:       cookiePath,
				Value:      string(value),
				IsSecure:   isSecure != 0,
				IsHTTPOnly: isHTTPOnly != 0,
				ExpireAt:   typeutil.TimeEpoch(expireAt),
				CreatedAt:  typeutil.TimeEpoch(createdAt),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(cookies, func(i, j int) bool {
		return cookies[i].CreatedAt.After(cookies[j].CreatedAt)
	})
	return cookies, nil
}
