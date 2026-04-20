package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	countLoginQuery   = `SELECT COUNT(*) FROM logins`
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

// extractYandexPasswords extracts passwords from Yandex's Ya Passman Data, which stores the URL in
// action_url instead of origin_url.
func extractYandexPasswords(keys keyretriever.MasterKeys, path string) ([]types.LoginEntry, error) {
	const yandexLoginQuery = `SELECT action_url, username_value, password_value, date_created FROM logins`
	return extractPasswordsWithQuery(keys, path, yandexLoginQuery)
}

func countPasswords(path string) (int, error) {
	return sqliteutil.CountRows(path, false, countLoginQuery)
}
