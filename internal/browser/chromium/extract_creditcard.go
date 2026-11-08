package chromium

import (
	"database/sql"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

const (
	defaultCreditCardQuery = `SELECT COALESCE(guid, ''), name_on_card, expiration_month, expiration_year,
		card_number_encrypted, COALESCE(nickname, ''), COALESCE(billing_address_id, '') FROM credit_cards`
)

func extractCreditCards(masterKeys masterKeysFunc, path string, out browser.Collector) error {
	return sqlitex.EachRow(path, defaultCreditCardQuery,
		func(rows *sql.Rows) error {
			var guid, name, month, year, nickname, address string
			var encryptedNumber []byte
			if err := rows.Scan(&guid, &name, &month, &year, &encryptedNumber, &nickname, &address); err != nil {
				return err
			}
			number, err := masterKeys.decrypt(encryptedNumber)
			if err != nil {
				out.Fail(fmt.Errorf("decrypt card %s: %w", guid, err))
			}
			out.Add(&record.CreditCard{
				GUID:     guid,
				Name:     name,
				Number:   string(number),
				ExpMonth: month,
				ExpYear:  year,
				NickName: nickname,
				Address:  address,
			})
			return nil
		}, out.Fail)
}
