package chromium

import (
	"database/sql"
	"errors"
	"sort"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	countLoginQuery   = `SELECT COUNT(*) FROM logins`

	yandexLoginQuery = `SELECT origin_url, username_element, username_value,
		password_element, password_value, signon_realm, date_created FROM logins`
)

func extractPasswords(keys keyretriever.MasterKeys, path string) ([]types.LoginEntry, error) {
	return extractPasswordsWithQuery(keys, path, defaultLoginQuery)
}

func extractPasswordsWithQuery(keys keyretriever.MasterKeys, path, query string) ([]types.LoginEntry, error) {
	logins, err := sqliteutil.QueryRows(path, false, query,
		func(rows *sql.Rows) (types.LoginEntry, error) {
			var url, username string
			var pwd []byte
			var created int64
			if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
				return types.LoginEntry{}, err
			}
			password, _ := decryptValue(keys, pwd)
			return types.LoginEntry{
				URL:       url,
				Username:  username,
				Password:  string(password),
				CreatedAt: timeEpoch(created),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(logins, func(i, j int) bool {
		return logins[i].CreatedAt.After(logins[j].CreatedAt)
	})
	return logins, nil
}

// extractYandexPasswords walks Ya Passman Data; protocol in RFC-012 §4.
// Note: URL column is origin_url — it's what the per-row AAD is computed over (not action_url).
func extractYandexPasswords(keys keyretriever.MasterKeys, path string) ([]types.LoginEntry, error) {
	dataKey, err := loadYandexDataKey(path, keys.V10)
	if err != nil {
		if errors.Is(err, errYandexMasterPasswordSet) {
			log.Warnf("%s: %v", path, err)
			return nil, nil
		}
		return nil, err
	}

	logins, err := sqliteutil.QueryRows(path, false, yandexLoginQuery,
		func(rows *sql.Rows) (types.LoginEntry, error) {
			var originURL, usernameElem, usernameVal, passwordElem, signonRealm string
			var passwordValue []byte
			var created int64
			if err := rows.Scan(&originURL, &usernameElem, &usernameVal, &passwordElem, &passwordValue, &signonRealm, &created); err != nil {
				return types.LoginEntry{}, err
			}
			entry := types.LoginEntry{
				URL:       originURL,
				Username:  usernameVal,
				CreatedAt: timeEpoch(created),
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

	sort.Slice(logins, func(i, j int) bool {
		return logins[i].CreatedAt.After(logins[j].CreatedAt)
	})
	return logins, nil
}

func countPasswords(path string) (int, error) {
	return sqliteutil.CountRows(path, false, countLoginQuery)
}
