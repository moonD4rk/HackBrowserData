package firefox

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	firefoxCookieQuery = `SELECT name, value, host, path,
		creationTime, expiry, isSecure, isHttpOnly, sameSite FROM moz_cookies`
	firefoxCountCookieQuery = `SELECT COUNT(*) FROM moz_cookies`
)

func extractCookies(path string) ([]types.CookieEntry, error) {
	cookies, err := sqliteutil.QueryRows(path, true, firefoxCookieQuery,
		func(rows *sql.Rows) (types.CookieEntry, error) {
			var (
				name, value, host, cookiePath string
				isSecure, isHTTPOnly          int
				createdAt, expiry             int64
				sameSite                      int
			)
			if err := rows.Scan(&name, &value, &host, &cookiePath,
				&createdAt, &expiry, &isSecure, &isHTTPOnly, &sameSite); err != nil {
				return types.CookieEntry{}, err
			}
			hasExpire := expiry > 0
			sameSiteStr := "unspecified"
			switch sameSite {
			case 0:
				sameSiteStr = "none"
			case 1:
				sameSiteStr = "lax"
			case 2:
				sameSiteStr = "strict"
			case 256:
				// not specified by Set-Cookie
			}
			return types.CookieEntry{
				Name:         name,
				Host:         host,
				Path:         cookiePath,
				Value:        value, // Firefox cookies are not encrypted
				IsSecure:     isSecure != 0,
				IsHTTPOnly:   isHTTPOnly != 0,
				HasExpire:    hasExpire,
				IsPersistent: hasExpire,
				ExpireAt:     firefoxSeconds(expiry),
				CreatedAt:    firefoxMicros(createdAt),
				SameSite:     sameSiteStr,
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

func countCookies(path string) (int, error) {
	return sqliteutil.CountRows(path, true, firefoxCountCookieQuery)
}
