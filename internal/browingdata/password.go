package browingdata

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	"hack-browser-data/internal/browser/item"

	decrypter2 "hack-browser-data/internal/decrypter"
	"hack-browser-data/internal/utils"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
)

type ChromiumPassword []loginData

func (c *ChromiumPassword) Parse(masterKey []byte) error {
	loginDB, err := sql.Open("sqlite3", item.TempChromiumPassword)
	if err != nil {
		return err
	}
	defer loginDB.Close()
	rows, err := loginDB.Query(queryChromiumLogin)
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
			fmt.Println(err)
		}
		login := loginData{
			UserName:    username,
			encryptPass: pwd,
			LoginUrl:    url,
		}
		if len(pwd) > 0 {
			if masterKey == nil {
				password, err = decrypter2.DPApi(pwd)
				if err != nil {
					fmt.Println(err)
				}
			} else {
				password, err = decrypter2.ChromePass(masterKey, pwd)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
		if create > time.Now().Unix() {
			login.CreateDate = utils.TimeEpochFormat(create)
		} else {
			login.CreateDate = utils.TimeStampFormat(create)
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

type FirefoxPassword []loginData

func (f *FirefoxPassword) Parse(masterKey []byte) error {
	globalSalt, metaBytes, nssA11, nssA102, err := getFirefoxDecryptKey(item.FirefoxKey4Filename)
	if err != nil {
		return err
	}
	metaPBE, err := decrypter2.NewASN1PBE(metaBytes)
	if err != nil {
		return err
	}

	k, err := metaPBE.Decrypt(globalSalt, masterKey)
	if err != nil {
		return err
	}
	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if bytes.Contains(k, []byte("password-check")) {
		m := bytes.Compare(nssA102, keyLin)
		if m == 0 {
			nssPBE, err := decrypter2.NewASN1PBE(nssA11)
			if err != nil {
				return err
			}
			finallyKey, err := nssPBE.Decrypt(globalSalt, masterKey)
			finallyKey = finallyKey[:24]
			if err != nil {
				return err
			}
			allLogin, err := getFirefoxLoginData(item.FirefoxPasswordFilename)
			if err != nil {
				return err
			}
			for _, v := range allLogin {
				userPBE, err := decrypter2.NewASN1PBE(v.encryptUser)
				if err != nil {
					return err
				}
				pwdPBE, err := decrypter2.NewASN1PBE(v.encryptPass)
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
					LoginUrl:   v.LoginUrl,
					UserName:   string(decrypter2.PKCS5UnPadding(user)),
					Password:   string(decrypter2.PKCS5UnPadding(pwd)),
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

func (f *FirefoxPassword) Name() string {
	return "password"
}

func getFirefoxDecryptKey(key4file string) (item1, item2, a11, a102 []byte, err error) {
	var (
		keyDB *sql.DB
	)
	keyDB, err = sql.Open("sqlite3", key4file)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer keyDB.Close()

	if err = keyDB.QueryRow(queryMetaData).Scan(&item1, &item2); err != nil {
		return nil, nil, nil, nil, err
	}

	if err = keyDB.QueryRow(queryNssPrivate).Scan(&a11, &a102); err != nil {
		return nil, nil, nil, nil, err
	}
	return item1, item2, a11, a102, nil
}

func getFirefoxLoginData(loginJson string) (l []loginData, err error) {
	s, err := ioutil.ReadFile(loginJson)
	if err != nil {
		return nil, err
	}
	h := gjson.GetBytes(s, "logins")
	if h.Exists() {
		for _, v := range h.Array() {
			var (
				m    loginData
				user []byte
				pass []byte
			)
			m.LoginUrl = v.Get("formSubmitURL").String()
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
			m.CreateDate = utils.TimeStampFormat(v.Get("timeCreated").Int() / 1000)
			l = append(l, m)
		}
	}
	return l, nil
}
