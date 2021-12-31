package data

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/pkg/decrypter"
	"hack-browser-data/utils"

	_ "github.com/mattn/go-sqlite3"
)

type ChromiumPassword []loginData

func (c *ChromiumPassword) Parse(masterKey []byte) error {
	loginDB, err := sql.Open("sqlite3", consts.ChromiumPasswordFilename)
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
				password, err = decrypter.DPApi(pwd)
				if err != nil {
					fmt.Println(err)
				}
			} else {
				password, err = decrypter.ChromePass(masterKey, pwd)
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

type firefoxPassword struct {
}

func (c *firefoxPassword) Parse(masterKey []byte) error {
	return nil
}
