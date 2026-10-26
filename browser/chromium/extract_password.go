package chromium

import (
	"database/sql"
	"errors"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
	"github.com/moond4rk/hackbrowserdata/masterkey"
)

const (
	accountLoginData  = `Login Data For Account`
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	countLoginQuery   = `SELECT COUNT(*) FROM logins`

	yandexLoginQuery = `SELECT origin_url, username_element, username_value,
		password_element, password_value, signon_realm, date_created FROM logins`
)

// appendPasswords merges another login DB into dst. Entries are kept even when an identical
// credential already exists — the Store field records which DB each one came from.
func appendPasswords(masterKeys masterkey.MasterKeys, path string, store record.Store, dst []record.Login) ([]record.Login, error) {
	logins, err := extractPasswords(masterKeys, path, store)
	if err != nil {
		return dst, err
	}
	return append(dst, logins...), nil
}

func extractPasswords(masterKeys masterkey.MasterKeys, path string, store record.Store) ([]record.Login, error) {
	return extractPasswordsWithQuery(masterKeys, path, store, defaultLoginQuery)
}

func extractPasswordsWithQuery(masterKeys masterkey.MasterKeys, path string, store record.Store, query string) ([]record.Login, error) {
	logins, err := sqlitex.QueryRows(path, false, query,
		func(rows *sql.Rows) (record.Login, error) {
			var url, username string
			var pwd []byte
			var created int64
			if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
				return record.Login{}, err
			}
			password, _ := decryptValue(masterKeys, pwd)
			return record.Login{
				URL:       url,
				Username:  username,
				Password:  string(password),
				CreatedAt: timeconv.Chromium(created),
				Store:     store,
			}, nil
		})
	if err != nil {
		return nil, err
	}
	return logins, nil
}

// extractYandexPasswords walks Ya Passman Data.
// Note: URL column is origin_url — it's what the per-row AAD is computed over (not action_url).
func extractYandexPasswords(masterKeys masterkey.MasterKeys, path string) ([]record.Login, error) {
	dataKey, err := loadYandexDataKey(path, masterKeys.V10)
	if err != nil {
		if errors.Is(err, errYandexMasterPasswordSet) {
			log.Warnf("%s: %v", path, err)
			return nil, nil
		}
		return nil, err
	}

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
			aad := yandexLoginAAD(originURL, usernameElem, usernameVal, passwordElem, signonRealm, nil)
			plaintext, err := crypto.AESGCMDecryptBlob(dataKey, passwordValue, aad)
			if err != nil {
				log.Debugf("yandex: decrypt password for %s: %v", originURL, err)
				return entry, nil
			}
			entry.Password = string(plaintext)
			return entry, nil
		})
	if err != nil {
		return nil, err
	}
	return logins, nil
}

func countPasswords(path string) (int, error) {
	return sqlitex.CountRows(path, false, countLoginQuery)
}
