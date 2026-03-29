package chromium

import (
	"database/sql"
	"strconv"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const defaultCreditCardQuery = `SELECT name_on_card, expiration_month, expiration_year,
	card_number_encrypted FROM credit_cards`

func extractCreditCards(masterKey []byte, path string) ([]types.CreditCardEntry, error) {
	return sqliteutil.QueryRows(path, false, defaultCreditCardQuery,
		func(rows *sql.Rows) (types.CreditCardEntry, error) {
			var name string
			var month, year int
			var encNumber []byte
			if err := rows.Scan(&name, &month, &year, &encNumber); err != nil {
				return types.CreditCardEntry{}, err
			}
			number, _ := decryptValue(masterKey, encNumber)
			return types.CreditCardEntry{
				Name:     name,
				Number:   string(number),
				ExpMonth: strconv.Itoa(month),
				ExpYear:  strconv.Itoa(year),
			}, nil
		})
}
