package chromium

import (
	"database/sql"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`
)

func extractPasswords(masterKeys masterKeysFunc, path string, store record.Store, out browser.Collector) error {
	return sqlitex.EachRow(path, defaultLoginQuery,
		func(rows *sql.Rows) error {
			var url, username string
			var encryptedPassword []byte
			var created int64
			if err := rows.Scan(&url, &username, &encryptedPassword, &created); err != nil {
				return err
			}
			password, err := masterKeys.decrypt(encryptedPassword)
			if err != nil {
				out.Fail(fmt.Errorf("decrypt password for %s: %w", url, err))
			}
			out.Add(&record.Login{
				URL:       url,
				Username:  username,
				Password:  string(password),
				CreatedAt: timeconv.Chromium(created),
				Store:     store,
			})
			return nil
		}, out.Fail)
}
