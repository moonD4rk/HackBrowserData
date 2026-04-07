package chromium

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	defaultCreditCardQuery = `SELECT COALESCE(guid, ''), name_on_card, expiration_month, expiration_year,
		card_number_encrypted, COALESCE(nickname, ''), COALESCE(billing_address_id, '') FROM credit_cards`
	countCreditCardQuery = `SELECT COUNT(*) FROM credit_cards`
)

func extractCreditCards(masterKey []byte, path string) ([]types.CreditCardEntry, error) {
	var decryptFails int
	var lastErr error
	cards, err := sqliteutil.QueryRows(path, false, defaultCreditCardQuery,
		func(rows *sql.Rows) (types.CreditCardEntry, error) {
			var guid, name, month, year, nickname, address string
			var encNumber []byte
			if err := rows.Scan(&guid, &name, &month, &year, &encNumber, &nickname, &address); err != nil {
				return types.CreditCardEntry{}, err
			}
			number, err := decryptValue(masterKey, encNumber)
			if err != nil {
				decryptFails++
				lastErr = err
			}
			return types.CreditCardEntry{
				GUID:     guid,
				Name:     name,
				Number:   string(number),
				ExpMonth: month,
				ExpYear:  year,
				NickName: nickname,
				Address:  address,
			}, nil
		})
	if err != nil {
		return nil, err
	}
	if decryptFails > 0 {
		log.Debugf("decrypt credit cards: %d failed: %v", decryptFails, lastErr)
	}
	return cards, nil
}

func countCreditCards(path string) (int, error) {
	return sqliteutil.CountRows(path, false, countCreditCardQuery)
}
