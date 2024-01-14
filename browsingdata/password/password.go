package password

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"log/slog"
	"os"
	"sort"
	"time"

	// import sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/item"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

type ChromiumPassword []loginData

type loginData struct {
	UserName    string
	encryptPass []byte
	encryptUser []byte
	Password    string
	LoginURL    string
	CreateDate  time.Time
}

const (
	queryChromiumLogin = `SELECT origin_url, username_value, password_value, date_created FROM logins`
)

func (c *ChromiumPassword) Parse(masterKey []byte) error {
	db, err := sql.Open("sqlite3", item.ChromiumPassword.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(item.ChromiumPassword.TempFilename())
	defer db.Close()

	rows, err := db.Query(queryChromiumLogin)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			url, username string
			pwd, password []byte
			create        int64
		)
		if err := rows.Scan(&url, &username, &pwd, &create); err != nil {
			slog.Error("scan chromium password error", "err", err)
		}
		login := loginData{
			UserName:    username,
			encryptPass: pwd,
			LoginURL:    url,
		}
		if len(pwd) > 0 {
			if len(masterKey) == 0 {
				password, err = crypto.DPAPI(pwd)
			} else {
				password, err = crypto.DecryptPass(masterKey, pwd)
			}
			if err != nil {
				slog.Error("decrypt chromium password error", "err", err)
			}
		}
		if create > time.Now().Unix() {
			login.CreateDate = typeutil.TimeEpoch(create)
		} else {
			login.CreateDate = typeutil.TimeStamp(create)
		}
		login.Password = string(password)
		*c = append(*c, login)
	}
	// sort with create date
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].CreateDate.After((*c)[j].CreateDate)
	})
	return nil
}

func (c *ChromiumPassword) Name() string {
	return "password"
}

func (c *ChromiumPassword) Len() int {
	return len(*c)
}

type YandexPassword []loginData

const (
	queryYandexLogin = `SELECT action_url, username_value, password_value, date_created FROM logins`
)

func (c *YandexPassword) Parse(masterKey []byte) error {
	db, err := sql.Open("sqlite3", item.YandexPassword.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(item.YandexPassword.TempFilename())
	defer db.Close()

	rows, err := db.Query(queryYandexLogin)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			url, username string
			pwd, password []byte
			create        int64
		)
		if err := rows.Scan(&url, &username, &pwd, &create); err != nil {
			slog.Error("scan yandex password error", "err", err)
		}
		login := loginData{
			UserName:    username,
			encryptPass: pwd,
			LoginURL:    url,
		}

		if len(pwd) > 0 {
			if len(masterKey) == 0 {
				password, err = crypto.DPAPI(pwd)
			} else {
				password, err = crypto.DecryptPass(masterKey, pwd)
			}
			if err != nil {
				slog.Error("decrypt yandex password error", "err", err)
			}
		}
		if create > time.Now().Unix() {
			login.CreateDate = typeutil.TimeEpoch(create)
		} else {
			login.CreateDate = typeutil.TimeStamp(create)
		}
		login.Password = string(password)
		*c = append(*c, login)
	}
	// sort with create date
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].CreateDate.After((*c)[j].CreateDate)
	})
	return nil
}

func (c *YandexPassword) Name() string {
	return "password"
}

func (c *YandexPassword) Len() int {
	return len(*c)
}

type FirefoxPassword []loginData

const (
	queryMetaData   = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	queryNssPrivate = `SELECT a11, a102 from nssPrivate`
)

func (f *FirefoxPassword) Parse(masterKey []byte) error {
	globalSalt, metaBytes, nssA11, nssA102, err := getFirefoxDecryptKey(item.FirefoxKey4.TempFilename())
	if err != nil {
		return err
	}
	metaPBE, err := crypto.NewASN1PBE(metaBytes)
	if err != nil {
		return err
	}

	k, err := metaPBE.Decrypt(globalSalt, masterKey)
	if err != nil {
		return err
	}
	if bytes.Contains(k, []byte("password-check")) {
		keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
		if bytes.Equal(nssA102, keyLin) {
			nssPBE, err := crypto.NewASN1PBE(nssA11)
			if err != nil {
				return err
			}
			finallyKey, err := nssPBE.Decrypt(globalSalt, masterKey)
			if err != nil {
				return err
			}

			finallyKey = finallyKey[:24]
			logins, err := getFirefoxLoginData()
			if err != nil {
				return err
			}

			for _, v := range logins {
				userPBE, err := crypto.NewASN1PBE(v.encryptUser)
				if err != nil {
					return err
				}
				pwdPBE, err := crypto.NewASN1PBE(v.encryptPass)
				if err != nil {
					return err
				}
				user, err := userPBE.Decrypt(finallyKey, masterKey)
				if err != nil {
					return err
				}
				pwd, err := pwdPBE.Decrypt(finallyKey, masterKey)
				if err != nil {
					return err
				}
				*f = append(*f, loginData{
					LoginURL:   v.LoginURL,
					UserName:   string(user),
					Password:   string(pwd),
					CreateDate: v.CreateDate,
				})
			}
		}
	}
	sort.Slice(*f, func(i, j int) bool {
		return (*f)[i].CreateDate.After((*f)[j].CreateDate)
	})
	return nil
}

func getFirefoxDecryptKey(key4file string) (item1, item2, a11, a102 []byte, err error) {
	keyDB, err := sql.Open("sqlite3", key4file)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer os.Remove(key4file)
	defer keyDB.Close()

	if err = keyDB.QueryRow(queryMetaData).Scan(&item1, &item2); err != nil {
		return nil, nil, nil, nil, err
	}

	if err = keyDB.QueryRow(queryNssPrivate).Scan(&a11, &a102); err != nil {
		return nil, nil, nil, nil, err
	}
	return item1, item2, a11, a102, nil
}

func getFirefoxLoginData() ([]loginData, error) {
	s, err := os.ReadFile(item.FirefoxPassword.TempFilename())
	if err != nil {
		return nil, err
	}
	defer os.Remove(item.FirefoxPassword.TempFilename())
	loginsJSON := gjson.GetBytes(s, "logins")
	var logins []loginData
	if loginsJSON.Exists() {
		for _, v := range loginsJSON.Array() {
			var (
				m    loginData
				user []byte
				pass []byte
			)
			m.LoginURL = v.Get("formSubmitURL").String()
			user, err = base64.StdEncoding.DecodeString(v.Get("encryptedUsername").String())
			if err != nil {
				return nil, err
			}
			pass, err = base64.StdEncoding.DecodeString(v.Get("encryptedPassword").String())
			if err != nil {
				return nil, err
			}
			m.encryptUser = user
			m.encryptPass = pass
			m.CreateDate = typeutil.TimeStamp(v.Get("timeCreated").Int() / 1000)
			logins = append(logins, m)
		}
	}
	return logins, nil
}

func (f *FirefoxPassword) Name() string {
	return "password"
}

func (f *FirefoxPassword) Len() int {
	return len(*f)
}
