package browingdata

import (
	"time"

	"hack-browser-data/internal/item"
)

type BrowsingData struct {
	sources map[item.Item]Source
}

type Source interface {
	Parse(masterKey []byte) error

	Name() string
}

func New(sources []item.Item) *BrowsingData {
	bd := &BrowsingData{
		sources: make(map[item.Item]Source),
	}
	bd.addSource(sources)
	return bd
}

func (b *BrowsingData) addSource(sources []item.Item) {
	for _, source := range sources {
		switch source {
		case item.ChromiumPassword:
			b.sources[source] = &ChromiumPassword{}
		case item.ChromiumCookie:
			b.sources[source] = &ChromiumCookie{}
		case item.ChromiumBookmark:
			b.sources[source] = &ChromiumBookmark{}
		case item.ChromiumHistory:
			b.sources[source] = &ChromiumHistory{}
		case item.ChromiumDownload:
			b.sources[source] = &ChromiumDownload{}
		case item.ChromiumCreditCard:
			b.sources[source] = &ChromiumCreditCard{}
		case item.FirefoxPassword:
			b.sources[source] = &FirefoxPassword{}
		case item.FirefoxCookie:
			b.sources[source] = &FirefoxCookie{}
		case item.FirefoxBookmark:
			b.sources[source] = &FirefoxBookmark{}
		case item.FirefoxHistory:
			b.sources[source] = &FirefoxHistory{}
		case item.FirefoxDownload:
			b.sources[source] = &FirefoxDownload{}
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
