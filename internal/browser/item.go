package browser

import (
	"hack-browser-data/internal/browser/consts"
	data2 "hack-browser-data/internal/browser/data"
)

type item int

const (
	chromiumKey item = iota
	chromiumPassword
	chromiumCookie
	chromiumBookmark
	chromiumHistory
	chromiumDownload
	chromiumCreditCard
	chromiumLocalStorage
	chromiumExtension

	yandexPassword
	yandexCreditCard

	firefoxKey4
	firefoxPassword
	firefoxCookie
	firefoxBookmark
	firefoxHistory
	firefoxDownload
	firefoxCreditCard
	firefoxLocalStorage
	firefoxExtension
)

func (i item) DefaultName() string {
	switch i {
	case chromiumKey:
		return consts.ChromiumKey
	case chromiumPassword:
		return consts.ChromiumPassword
	case chromiumCookie:
		return consts.ChromiumCookie
	case chromiumBookmark:
		return consts.ChromiumBookmark
	case chromiumDownload:
		return consts.ChromiumDownload
	case chromiumLocalStorage:
		return consts.ChromiumLocalStorage
	case chromiumCreditCard:
		return consts.ChromiumCredit
	case chromiumExtension:
		return consts.UnknownItem
	case chromiumHistory:
		return consts.ChromiumHistory
	case yandexPassword:
		return consts.YandexPassword
	case yandexCreditCard:
		return consts.YandexCredit
	case firefoxKey4:
		return consts.FirefoxKey4
	case firefoxPassword:
		return consts.FirefoxPassword
	case firefoxCookie:
		return consts.FirefoxCookie
	case firefoxBookmark:
		return consts.FirefoxData
	case firefoxDownload:
		return consts.FirefoxData
	case firefoxLocalStorage:
		return consts.UnsupportedItem
	case firefoxCreditCard:
		return consts.UnsupportedItem
	case firefoxHistory:
		return consts.FirefoxData
	case firefoxExtension:
		return consts.UnsupportedItem
	default:
		return consts.UnknownItem
	}
}

func (i item) FileName() string {
	switch i {
	case chromiumKey:
		return consts.ChromiumKeyFilename
	case chromiumPassword:
		return consts.ChromiumPasswordFilename
	case chromiumCookie:
		return consts.ChromiumCookieFilename
	case chromiumBookmark:
		return consts.ChromiumBookmarkFilename
	case chromiumDownload:
		return consts.ChromiumDownloadFilename
	case chromiumLocalStorage:
		return consts.ChromiumLocalStorageFilename
	case chromiumCreditCard:
		return consts.ChromiumCreditFilename
	case chromiumHistory:
		return consts.ChromiumHistoryFilename
	case chromiumExtension:
		return consts.UnsupportedItem
	case yandexPassword:
		return consts.ChromiumPasswordFilename
	case yandexCreditCard:
		return consts.ChromiumCreditFilename
	case firefoxKey4:
		return consts.FirefoxKey4Filename
	case firefoxPassword:
		return consts.FirefoxPasswordFilename
	case firefoxCookie:
		return consts.FirefoxCookieFilename
	case firefoxBookmark:
		return consts.FirefoxBookmarkFilename
	case firefoxDownload:
		return consts.FirefoxDownloadFilename
	case firefoxLocalStorage:
		return consts.UnsupportedItem
	case firefoxCreditCard:
		return consts.UnsupportedItem
	case firefoxHistory:
		return consts.FirefoxHistoryFilename
	case firefoxExtension:
		return consts.UnsupportedItem
	default:
		return consts.UnknownItem
	}
}

func (i item) NewBrowsingData() data2.BrowsingData {
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
