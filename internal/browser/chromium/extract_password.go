package chromium

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`
)

// accountLoginData holds the signed-in account's logins, which Chrome keeps apart from Login Data.
const accountLoginData = "Login Data For Account"

func extractPasswords(masterKeys masterKeysFunc, path string, out browser.Collector) error {
	store := record.StoreLocal
	if filepath.Base(path) == accountLoginData {
		store = record.StoreAccount
	}
	logins, err := sqlitex.QueryRows(path, false, defaultLoginQuery,
		func(rows *sql.Rows) (record.Login, error) {
			var url, username string
			var encryptedPassword []byte
			var created int64
			if err := rows.Scan(&url, &username, &encryptedPassword, &created); err != nil {
				return record.Login{}, err
			}
			password, err := masterKeys.decrypt(encryptedPassword)
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
	browser.AddAll(out, logins)
	return nil
}
