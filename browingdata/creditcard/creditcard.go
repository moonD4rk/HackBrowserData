package creditcard

import (
	"database/sql"
	"os"

	// import sqlite3 driver
	_ "github.com/mattn/go-sqlite3"

	"github.com/moond4rk/HackBrowserData/crypto"
	"github.com/moond4rk/HackBrowserData/item"
	"github.com/moond4rk/HackBrowserData/log"
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
	db, err := sql.Open("sqlite3", item.TempChromiumCreditCard)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempChromiumCreditCard)
	defer db.Close()

	rows, err := db.Query(queryChromiumCredit)
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
		if len(encryptValue) > 0 {
			if len(masterKey) == 0 {
				value, err = crypto.DPAPI(encryptValue)
			} else {
				value, err = crypto.DecryptPass(masterKey, encryptValue)
			}
			if err != nil {
				log.Error(err)
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

func (c *ChromiumCreditCard) Len() int {
	return len(*c)
}

type YandexCreditCard []card

func (c *YandexCreditCard) Parse(masterKey []byte) error {
	db, err := sql.Open("sqlite3", item.TempYandexCreditCard)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempYandexCreditCard)
	defer db.Close()
	rows, err := db.Query(queryChromiumCredit)
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
		if len(encryptValue) > 0 {
			if len(masterKey) == 0 {
				value, err = crypto.DPAPI(encryptValue)
			} else {
				value, err = crypto.DecryptPass(masterKey, encryptValue)
			}
			if err != nil {
				log.Error(err)
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

func (c *YandexCreditCard) Len() int {
	return len(*c)
}
