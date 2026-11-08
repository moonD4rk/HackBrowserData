package chromium

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const cookieQuery = `SELECT name, encrypted_value, host_key, path,
	creation_utc, expires_utc, is_secure, is_httponly,
	has_expires, is_persistent, samesite FROM cookies`

func extractCookies(masterKeys masterKeysFunc, path string, out browser.Collector) error {
	return sqlitex.EachRow(path, cookieQuery,
		func(rows *sql.Rows) error {
			var (
				name, host, cookiePath  string
				isSecure, isHTTPOnly    int
				hasExpire, isPersistent int
				createdAt, expireAt     int64
				encryptedValue          []byte
				sameSite                int
			)
			if err := rows.Scan(&name, &encryptedValue, &host, &cookiePath,
				&createdAt, &expireAt, &isSecure, &isHTTPOnly,
				&hasExpire, &isPersistent, &sameSite); err != nil {
				return err
			}

			value, err := masterKeys.decrypt(encryptedValue)
			if err != nil {
				out.Fail(fmt.Errorf("decrypt cookie %s on %s: %w", name, host, err))
			}
			value = stripCookieHash(value, host)
			sameSiteName := "unspecified"
			switch sameSite {
			case 0:
				sameSiteName = "none"
			case 1:
				sameSiteName = "lax"
			case 2:
				sameSiteName = "strict"
			case -1:
				// not specified by Set-Cookie
			}
			out.Add(&record.Cookie{
				Name:         name,
				Host:         host,
				Path:         cookiePath,
				Value:        string(value),
				IsSecure:     isSecure != 0,
				IsHTTPOnly:   isHTTPOnly != 0,
				HasExpire:    hasExpire != 0,
				IsPersistent: isPersistent != 0,
				ExpireAt:     timeconv.Chromium(expireAt),
				CreatedAt:    timeconv.Chromium(createdAt),
				SameSite:     sameSiteName,
			})
			return nil
		}, out.Fail)
}

// stripCookieHash removes the SHA256(host_key) Chrome 130+ prepends; a value without that prefix is returned as is.
func stripCookieHash(value []byte, hostKey string) []byte {
	if len(value) < sha256.Size {
		return value
	}
	hash := sha256.Sum256([]byte(hostKey))
	if bytes.Equal(value[:sha256.Size], hash[:]) {
		return value[sha256.Size:] // empty slice if value was exactly 32 bytes
	}
	return value
}
