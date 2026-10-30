package chromium

import (
	"database/sql"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`

	yandexLoginQuery = `SELECT origin_url, username_element, username_value,
		password_element, password_value, signon_realm, date_created FROM logins`
)

func extractPasswords(masterKeys masterkey.Set, path string, store record.Store, out browser.Collector) error {
	return extractPasswordsWithQuery(masterKeys, path, store, defaultLoginQuery, out)
}

func extractPasswordsWithQuery(masterKeys masterkey.Set, path string, store record.Store, query string, out browser.Collector) error {
	logins, err := sqlitex.QueryRows(path, false, query,
		func(rows *sql.Rows) (record.Login, error) {
			var url, username string
			var pwd []byte
			var created int64
			if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
				return record.Login{}, err
			}
			password, err := decryptValue(masterKeys, pwd)
			if err != nil {
				out.Fail(fmt.Errorf("decrypt password for %s: %w", url, err))
			}
			return record.Login{
				URL:       url,
				Username:  username,
				Password:  string(password),
				CreatedAt: timeconv.Chromium(created),
				Store:     store,
			}, nil
		})
	if err != nil {
		return err
	}
	emit(out, logins)
	return nil
}

// extractYandexPasswords walks Ya Passman Data.
// Note: URL column is origin_url — it's what the per-row AAD is computed over (not action_url).
func extractYandexPasswords(masterKeys masterkey.Set, path string, out browser.Collector) error {
	dataKey, keyErr := loadYandexDataKey(path, masterKeys[masterkey.V10])

	logins, err := sqlitex.QueryRows(path, false, yandexLoginQuery,
		func(rows *sql.Rows) (record.Login, error) {
			var originURL, usernameElem, usernameVal, passwordElem, signonRealm string
			var passwordValue []byte
			var created int64
			if err := rows.Scan(&originURL, &usernameElem, &usernameVal, &passwordElem, &passwordValue, &signonRealm, &created); err != nil {
				return record.Login{}, err
			}
			entry := record.Login{
				URL:       originURL,
				Username:  usernameVal,
				CreatedAt: timeconv.Chromium(created),
				Store:     record.StoreLocal,
			}
			if dataKey == nil {
				out.Fail(fmt.Errorf("yandex: password for %s: %w", originURL, keyErr))
				return entry, nil
			}
			aad := yandexLoginAAD(originURL, usernameElem, usernameVal, passwordElem, signonRealm, nil)
			plaintext, err := crypto.AESGCMDecryptBlob(dataKey, passwordValue, aad)
			if err != nil {
				out.Fail(fmt.Errorf("yandex: decrypt password for %s: %w", originURL, err))
				return entry, nil
			}
			entry.Password = string(plaintext)
			return entry, nil
		})
	if err != nil {
		return err
	}
	emit(out, logins)
	return nil
}
