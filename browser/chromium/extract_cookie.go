package chromium

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	defaultCookieQuery = `SELECT name, encrypted_value, host_key, path,
		creation_utc, expires_utc, is_secure, is_httponly,
		has_expires, is_persistent FROM cookies`
	countCookieQuery = `SELECT COUNT(*) FROM cookies`
)

func extractCookies(keys keyretriever.MasterKeys, path string) ([]types.CookieEntry, error) {
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

			value, _ := decryptValue(keys, encryptedValue)
			value = stripCookieHash(value, host)
			return types.CookieEntry{
				Name:         name,
				Host:         host,
				Path:         cookiePath,
				Value:        string(value),
				IsSecure:     isSecure != 0,
				IsHTTPOnly:   isHTTPOnly != 0,
				HasExpire:    hasExpire != 0,
				IsPersistent: isPersistent != 0,
				ExpireAt:     timeEpoch(expireAt),
				CreatedAt:    timeEpoch(createdAt),
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
	return sqliteutil.CountRows(path, false, countCookieQuery)
}

// stripCookieHash removes the SHA256(host_key) prefix from a decrypted cookie value. Chrome 130+
// (Cookie DB schema version 24) prepends SHA256(domain) to the cookie value before encryption to
// prevent cross-domain cookie replay attacks. If the first 32 bytes don't match SHA256(hostKey), the
// value is returned unchanged, which handles both older Chrome versions and tampered data.
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
