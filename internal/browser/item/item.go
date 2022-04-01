package item

import (
	data2 "hack-browser-data/internal/browser/data"
)

type Item int

const (
	ItemChromiumKey Item = iota
	ItemChromiumPassword
	ItemChromiumCookie
	ItemChromiumBookmark
	ItemChromiumHistory
	ItemChromiumDownload
	ItemChromiumCreditCard
	ItemChromiumLocalStorage
	ItemChromiumExtension

	ItemYandexPassword
	ItemYandexCreditCard

	ItemFirefoxKey4
	ItemFirefoxPassword
	ItemFirefoxCookie
	ItemFirefoxBookmark
	ItemFirefoxHistory
	ItemFirefoxDownload
	ItemFirefoxCreditCard
	ItemFirefoxLocalStorage
	ItemFirefoxExtension
)

func (i Item) DefaultName() string {
	switch i {
	case ItemChromiumKey:
		return ChromiumKey
	case ItemChromiumPassword:
		return ChromiumPassword
	case ItemChromiumCookie:
		return ChromiumCookie
	case ItemChromiumBookmark:
		return ChromiumBookmark
	case ItemChromiumDownload:
		return ChromiumDownload
	case ItemChromiumLocalStorage:
		return ChromiumLocalStorage
	case ItemChromiumCreditCard:
		return ChromiumCredit
	case ItemChromiumExtension:
		return UnknownItem
	case ItemChromiumHistory:
		return ChromiumHistory
	case ItemYandexPassword:
		return YandexPassword
	case ItemYandexCreditCard:
		return YandexCredit
	case ItemFirefoxKey4:
		return FirefoxKey4
	case ItemFirefoxPassword:
		return FirefoxPassword
	case ItemFirefoxCookie:
		return FirefoxCookie
	case ItemFirefoxBookmark:
		return FirefoxData
	case ItemFirefoxDownload:
		return FirefoxData
	case ItemFirefoxLocalStorage:
		return UnsupportedItem
	case ItemFirefoxCreditCard:
		return UnsupportedItem
	case ItemFirefoxHistory:
		return FirefoxData
	case ItemFirefoxExtension:
		return UnsupportedItem
	default:
		return UnknownItem
	}
}

func (i Item) FileName() string {
	switch i {
	case chromiumKey:
		return TempChromiumKey
	case chromiumPassword:
		return TempChromiumPassword
	case chromiumCookie:
		return ChromiumCookieFilename
	case chromiumBookmark:
		return ChromiumBookmarkFilename
	case chromiumDownload:
		return ChromiumDownloadFilename
	case chromiumLocalStorage:
		return ChromiumLocalStorageFilename
	case chromiumCreditCard:
		return TempChromiumCredit
	case chromiumHistory:
		return TempChromiumHistory
	case chromiumExtension:
		return UnsupportedItem
	case yandexPassword:
		return TempChromiumPassword
	case yandexCreditCard:
		return TempChromiumCredit
	case firefoxKey4:
		return FirefoxKey4Filename
	case firefoxPassword:
		return FirefoxPasswordFilename
	case firefoxCookie:
		return FirefoxCookieFilename
	case firefoxBookmark:
		return FirefoxBookmarkFilename
	case firefoxDownload:
		return FirefoxDownloadFilename
	case firefoxLocalStorage:
		return UnsupportedItem
	case firefoxCreditCard:
		return UnsupportedItem
	case firefoxHistory:
		return FirefoxHistoryFilename
	case firefoxExtension:
		return UnsupportedItem
	default:
		return UnknownItem
	}
}

func (i Item) NewBrowsingData() data2.BrowsingData {
	switch i {
	case chromiumKey:
		return nil
	case chromiumPassword:
		return &data2.ChromiumPassword{}
	case chromiumCookie:
		return &data2.ChromiumCookie{}
	case chromiumBookmark:
		return &data2.ChromiumBookmark{}
	case chromiumDownload:
		return &data2.ChromiumDownload{}
	case chromiumLocalStorage:
		return nil
	case chromiumCreditCard:
		return &data2.ChromiumCreditCard{}
	case chromiumExtension:
		return nil
	case chromiumHistory:
		return &data2.ChromiumHistory{}
	case yandexPassword:
		return &data2.ChromiumPassword{}
	case yandexCreditCard:
		return &data2.ChromiumCreditCard{}
	case firefoxPassword:
		return &data2.FirefoxPassword{}
	case firefoxCookie:
		return &data2.FirefoxCookie{}
	case firefoxBookmark:
		return &data2.FirefoxBookmark{}
	case firefoxDownload:
		return &data2.FirefoxDownload{}
	case firefoxHistory:
		return &data2.FirefoxHistory{}
	default:
		return nil
	}
}
