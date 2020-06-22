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
)

const (
	bookmarkID       = "id"
	bookmarkAdded    = "date_added"
	bookmarkUrl      = "url"
	bookmarkName     = "name"
	bookmarkType     = "type"
	bookmarkChildren = "children"
)

type (
	BrowserData struct {
		BrowserName string
		LoginData   []*LoginData
		Bookmarks   []*Bookmarks
	}
	LoginData struct {
		UserName    string
		encryptPass []byte
		Password    string
		LoginUrl    string
		CreateDate  time.Time
	}
	Bookmarks struct {
		ID        string    `json:"id"`
		DateAdded time.Time `json:"date_added"`
		URL       string    `json:"url"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
	}
	Cookie struct {
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
			getBookmarkValue(value)
			return true
		})
		fmt.Println(len(bookmarkList))
	}
}

func getBookmarkValue(value gjson.Result) (children gjson.Result) {
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
				children = getBookmarkValue(v)
			}
		}
	}
	return children
}

func parseLogin() (results []*LoginData, err error) {
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
	rows, err := logins.Query(`SELECT origin_url, username_value, password_value, date_created FROM logins`)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		var (
			url      string
			username string
			pwd      []byte
			password string
			create   int64
		)
		err = rows.Scan(&url, &username, &pwd, &create)
		loginD = &LoginData{
			UserName:    username,
			encryptPass: pwd,
			LoginUrl:    url,
			CreateDate:  utils.TimeEpochFormat(create),
		}
		if len(pwd) > 3 {
			if err != nil {
				log.Println(err)
				continue
			}
			password, err = utils.Aes128CBCDecrypt(pwd[3:])
		}
		loginD.Password = password
		if err != nil {
			log.Println(err)
		}
		results = append(results, loginD)
	}
	return
}

func parseHistory() {

}

func parseCookie() {

}
