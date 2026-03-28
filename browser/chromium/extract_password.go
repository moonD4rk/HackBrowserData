package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

const defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func extractPasswords(masterKey []byte, path, query string) ([]types.LoginEntry, error) {
	if query == "" {
		query = defaultLoginQuery
	}

	logins, err := sqliteutil.QueryRows(path, false, query,
		func(rows *sql.Rows) (types.LoginEntry, error) {
			var url, username string
			var pwd []byte
			var created int64
			if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
				return types.LoginEntry{}, err
			}
			password, _ := decryptValue(masterKey, pwd)
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
