package chromium

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

const (
	defaultCreditCardQuery = `SELECT COALESCE(guid, ''), name_on_card, expiration_month, expiration_year,
		card_number_encrypted, COALESCE(nickname, ''), COALESCE(billing_address_id, '') FROM credit_cards`
	countCreditCardQuery = `SELECT COUNT(*) FROM credit_cards`

	yandexCreditCardQuery      = `SELECT guid, public_data, private_data FROM records`
	yandexCreditCardCountQuery = `SELECT COUNT(*) FROM records`
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

func extractCreditCards(masterKeys masterkey.Set, path string) ([]record.CreditCard, error) {
	cards, err := sqlitex.QueryRows(path, false, defaultCreditCardQuery,
		func(rows *sql.Rows) (record.CreditCard, error) {
			var guid, name, month, year, nickname, address string
			var encNumber []byte
			if err := rows.Scan(&guid, &name, &month, &year, &encNumber, &nickname, &address); err != nil {
				return record.CreditCard{}, err
			}
			number, _ := decryptValue(masterKeys, encNumber)
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
		return nil, err
	}
	return cards, nil
}

// extractYandexCreditCards reads the records table (not Chromium's credit_cards). AAD = guid.
func extractYandexCreditCards(masterKeys masterkey.Set, path string) ([]record.CreditCard, error) {
	dataKey, err := loadYandexDataKey(path, masterKeys[masterkey.V10])
	if err != nil {
		if errors.Is(err, errYandexMasterPasswordSet) {
			log.Warnf("%s: %v", path, err)
			return nil, nil
		}
		return nil, err
	}

	return sqlitex.QueryRows(path, false, yandexCreditCardQuery,
		func(rows *sql.Rows) (record.CreditCard, error) {
			var guid, publicData string
			var privateData []byte
			if err := rows.Scan(&guid, &publicData, &privateData); err != nil {
				return record.CreditCard{}, err
			}

			var public yandexPublicData
			if publicData != "" {
				if err := json.Unmarshal([]byte(publicData), &public); err != nil {
					log.Debugf("yandex: parse public_data for %s: %v", guid, err)
				}
			}
			entry := record.CreditCard{
				GUID:     guid,
				Name:     public.CardHolder,
				ExpMonth: public.ExpireDateMonth,
				ExpYear:  public.ExpireDateYear,
				NickName: public.CardTitle,
			}

			plaintext, err := crypto.AESGCMDecryptBlob(dataKey, privateData, yandexCardAAD(guid, nil))
			if err != nil {
				log.Debugf("yandex: decrypt card %s: %v", guid, err)
				return entry, nil
			}

			var private yandexPrivateData
			if err := json.Unmarshal(plaintext, &private); err != nil {
				log.Debugf("yandex: parse private_data for %s: %v", guid, err)
				return entry, nil
			}
			entry.Number = private.FullCardNumber
			entry.CVC = private.PinCode
			entry.Comment = private.SecretComment
			return entry, nil
		})
}

func countCreditCards(path string) (int, error) {
	return sqlitex.CountRows(path, false, countCreditCardQuery)
}

func countYandexCreditCards(path string) (int, error) {
	return sqlitex.CountRows(path, false, yandexCreditCardCountQuery)
}
