package browingdata

import (
	"time"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
)

type Data struct {
	Sources map[item.Item]Source
}

type Source interface {
	Parse(masterKey []byte) error

	Name() string
}

func New(sources []item.Item) *Data {
	bd := &Data{
		Sources: make(map[item.Item]Source),
	}
	bd.addSource(sources)
	return bd
}

func (d *Data) Recovery(masterKey []byte) error {

	for _, source := range d.Sources {
		if err := source.Parse(masterKey); err != nil {
			log.Error(err)
		}
	}
	return nil
}

func (d *Data) addSource(Sources []item.Item) {
	for _, source := range Sources {
		switch source {
		case item.ChromiumPassword:
			d.Sources[source] = &ChromiumPassword{}
		case item.ChromiumCookie:
			d.Sources[source] = &ChromiumCookie{}
		case item.ChromiumBookmark:
			d.Sources[source] = &ChromiumBookmark{}
		case item.ChromiumHistory:
			d.Sources[source] = &ChromiumHistory{}
		case item.ChromiumDownload:
			d.Sources[source] = &ChromiumDownload{}
		case item.ChromiumCreditCard:
			d.Sources[source] = &ChromiumCreditCard{}
		case item.FirefoxPassword:
			d.Sources[source] = &FirefoxPassword{}
		case item.FirefoxCookie:
			d.Sources[source] = &FirefoxCookie{}
		case item.FirefoxBookmark:
			d.Sources[source] = &FirefoxBookmark{}
		case item.FirefoxHistory:
			d.Sources[source] = &FirefoxHistory{}
		case item.FirefoxDownload:
			d.Sources[source] = &FirefoxDownload{}
		}
	}
}

const (
	queryChromiumCredit   = `SELECT guid, name_on_card, expiration_month, expiration_year, card_number_encrypted FROM credit_cards`
	queryChromiumLogin    = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	queryChromiumHistory  = `SELECT url, title, visit_count, last_visit_time FROM urls`
	queryChromiumDownload = `SELECT target_path, tab_url, total_bytes, start_time, end_time, mime_type FROM downloads`
	queryChromiumCookie   = `SELECT name, encrypted_value, host_key, path, creation_utc, expires_utc, is_secure, is_httponly, has_expires, is_persistent FROM cookies`
	queryFirefoxHistory   = `SELECT id, url, last_visit_date, title, visit_count FROM moz_places where title not null`
	queryFirefoxDownload  = `SELECT place_id, GROUP_CONCAT(content), url, dateAdded FROM (SELECT * FROM moz_annos INNER JOIN moz_places ON moz_annos.place_id=moz_places.id) t GROUP BY place_id`
	queryFirefoxBookMark  = `SELECT id, url, type, dateAdded, title FROM (SELECT * FROM moz_bookmarks INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id)`
	queryFirefoxCookie    = `SELECT name, value, host, path, creationTime, expiry, isSecure, isHttpOnly FROM moz_cookies`
	queryMetaData         = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	queryNssPrivate       = `SELECT a11, a102 from nssPrivate`
	closeJournalMode      = `PRAGMA journal_mode=off`
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
		MimeType   string
	}
	card struct {
		GUID            string
		Name            string
		ExpirationYear  string
		ExpirationMonth string
		CardNumber      string
	}
)
