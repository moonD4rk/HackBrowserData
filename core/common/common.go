package common

import (
	"database/sql"
	"fmt"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
)

const (
	Chrome = "Chrome"
	Safari = "Safari"
)

var (
	browserData  = new(BrowserData)
	bookmarkList []*Bookmarks
	cookieList   []*Cookies
)

const (
	bookmarkID       = "id"
	bookmarkAdded    = "date_added"
	bookmarkUrl      = "url"
	bookmarkName     = "name"
	bookmarkType     = "type"
	bookmarkChildren = "children"
)

const (
	queryHistory = ``
)

type (
	BrowserData struct {
		BrowserName string
		LoginData   []*LoginData
		Bookmarks   []*Bookmarks
	}
	LoginData struct {
		UserName    string    `json:"user_name"`
		encryptPass []byte    `json:"-"`
		Password    string    `json:"password"`
		LoginUrl    string    `json:"login_url"`
		CreateDate  time.Time `json:"create_date"`
	}
	Bookmarks struct {
		ID        string    `json:"id"`
		DateAdded time.Time `json:"date_added"`
		URL       string    `json:"url"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
	}
	Cookies struct {
		KeyName      string
		encryptValue []byte
		Value        string
		Host         string
		Path         string
		IsSecure     bool
		IsHTTPOnly   bool
		HasExpire    bool
		IsPersistent bool
		CreateDate   time.Time
		ExpireDate   time.Time
	}
	History struct {
	}
)

func ParseDB(dbname string) {
	switch dbname {
	case utils.LoginData:
		r, err := parseLogin()
		if err != nil {
			fmt.Println(err)
		}
		for _, v := range r {
			fmt.Printf("%+v\n", v)
		}
	case utils.Bookmarks:
		parseBookmarks()
	case utils.Cookies:
		parseCookie()
	}

}

func parseBookmarks() {
	bookmarks, err := utils.ReadFile(utils.Bookmarks)
	if err != nil {
		log.Println(err)
	}
	r := gjson.Parse(bookmarks)
	if r.Exists() {
		roots := r.Get("roots")
		roots.ForEach(func(key, value gjson.Result) bool {
			getBookmarkChildren(value)
			return true
		})
		fmt.Println(len(bookmarkList))
	}
}

var queryLogin = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func parseLogin() (results []*LoginData, err error) {
	//datetime(visit_time / 1000000 + (strftime('%s', '1601-01-01')), 'unixepoch')
	login := &LoginData{}
	loginDB, err := sql.Open("sqlite3", utils.LoginData)
	defer func() {
		if err := loginDB.Close(); err != nil {
			log.Println(err)
		}
	}()
	if err != nil {
		log.Println(err)
	}
	err = loginDB.Ping()
	rows, err := loginDB.Query(queryLogin)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		var (
			url, username, password string
			pwd                     []byte
			create                  int64
		)
		err = rows.Scan(&url, &username, &pwd, &create)
		login = &LoginData{
			UserName:    username,
			encryptPass: pwd,
			LoginUrl:    url,
			CreateDate:  utils.TimeEpochFormat(create),
		}
		if len(pwd) > 3 {
			// remove prefix 'v10'
			password, err = utils.Aes128CBCDecrypt(pwd[3:])
		}
		login.Password = password
		if err != nil {
			log.Println(err)
		}
		results = append(results, login)
	}
	return
}

var queryCookie = `SELECT name, encrypted_value, host_key, path, creation_utc, expires_utc, is_secure, is_httponly, has_expires, is_persistent FROM cookies`
func parseCookie() (results []*Cookies, err error) {
	cookies := &Cookies{}
	cookieDB, err := sql.Open("sqlite3", utils.Cookies)
	defer func() {
		if err := cookieDB.Close(); err != nil {
			log.Println(err)
		}
	}()
	if err != nil {
		log.Println(err)
	}
	err = cookieDB.Ping()
	rows, err := cookieDB.Query(queryCookie)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		var (
			key, host, path, value                        string
			isSecure, isHTTPOnly, hasExpire, isPersistent bool
			createDate, expireDate                        int64
			encryptValue                                  []byte
		)
		err = rows.Scan(&key, &encryptValue, &host, &path, &createDate, &expireDate, &isSecure, &isHTTPOnly, &hasExpire, &isPersistent)
		cookies = &Cookies{
			KeyName:      key,
			Host:         host,
			Path:         path,
			encryptValue: encryptValue,
			IsSecure:     false,
			IsHTTPOnly:   false,
			HasExpire:    false,
			IsPersistent: isPersistent,
			CreateDate:   utils.TimeEpochFormat(createDate),
			ExpireDate:   utils.TimeEpochFormat(expireDate),
		}
		if len(encryptValue) > 3 {
			// remove prefix 'v10'
			value, err = utils.Aes128CBCDecrypt(encryptValue[3:])
		}
		cookies.Value = value
		cookieList = append(cookieList, cookies)
	}
	return cookieList, err
}

func parseHistory() {

}

func getBookmarkChildren(value gjson.Result) (children gjson.Result) {
	b := new(Bookmarks)
	b.ID = value.Get(bookmarkID).String()
	nodeType := value.Get(bookmarkType)
	b.DateAdded = utils.TimeEpochFormat(value.Get(bookmarkAdded).Int())
	b.URL = value.Get(bookmarkUrl).String()
	b.Name = value.Get(bookmarkName).String()
	children = value.Get(bookmarkChildren)
	if nodeType.Exists() {
		b.Type = nodeType.String()
		bookmarkList = append(bookmarkList, b)
		if children.Exists() && children.IsArray() {
			for _, v := range children.Array() {
				children = getBookmarkChildren(v)
			}
		}
	}
	return children
}
