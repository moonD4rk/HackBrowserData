package firefox

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const firefoxCookieQuery = `SELECT name, value, host, path,
	creationTime, expiry, isSecure, isHttpOnly, sameSite FROM moz_cookies`

func extractCookies(path string, out browser.Collector) error {
	cookies, err := sqlitex.QueryRows(path, true, firefoxCookieQuery,
		func(rows *sql.Rows) (record.Cookie, error) {
			var (
				name, value, host, cookiePath string
				isSecure, isHTTPOnly          int
				createdAt, expiry             int64
				sameSite                      int
			)
			if err := rows.Scan(&name, &value, &host, &cookiePath,
				&createdAt, &expiry, &isSecure, &isHTTPOnly, &sameSite); err != nil {
				return record.Cookie{}, err
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
			return record.Cookie{
				Name:         name,
				Host:         host,
				Path:         cookiePath,
				Value:        value, // Firefox cookies are not encrypted
				IsSecure:     isSecure != 0,
				IsHTTPOnly:   isHTTPOnly != 0,
				HasExpire:    hasExpire,
				IsPersistent: hasExpire,
				ExpireAt:     timeconv.Unix(expiry),
				CreatedAt:    timeconv.UnixMicro(createdAt),
				SameSite:     sameSiteStr,
			}, nil
		})
	if err != nil {
		return err
	}
	emit(out, cookies)
	return nil
}
