package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

const defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
	return extractPasswordsWithQuery(masterKey, path, defaultLoginQuery)
}

func extractPasswordsWithQuery(masterKey []byte, path, query string) ([]types.LoginEntry, error) {
	logins, err := sqliteutil.QueryRows(path, false, query,
		func(rows *sql.Rows) (types.LoginEntry, error) {
			var url, username string
			var pwd []byte
			var created int64
			if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
				return types.LoginEntry{}, err
			}
			password, err := decryptValue(masterKey, pwd)
			if err != nil {
				log.Debugf("decrypt password for %s: %v", url, err)
			}
			return types.LoginEntry{
				URL:       url,
				Username:  username,
				Password:  string(password),
				CreatedAt: typeutil.TimeEpoch(created),
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

// extractYandexPasswords extracts passwords from Yandex's Ya Passman Data,
// which stores the URL in action_url instead of origin_url.
func extractYandexPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
	const yandexLoginQuery = `SELECT action_url, username_value, password_value, date_created FROM logins`
	return extractPasswordsWithQuery(masterKey, path, yandexLoginQuery)
}
