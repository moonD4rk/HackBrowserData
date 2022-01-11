package browser

import (
	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/pkg/browser/data"
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
		return consts.UnSupportItem
	case firefoxCreditCard:
		return consts.UnSupportItem
	case firefoxHistory:
		return consts.FirefoxData
	case firefoxExtension:
		return consts.UnSupportItem
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
		return consts.UnSupportItem
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
		return consts.UnSupportItem
	case firefoxCreditCard:
		return consts.UnSupportItem
	case firefoxHistory:
		return consts.FirefoxHistoryFilename
	case firefoxExtension:
		return consts.UnSupportItem
	default:
		return consts.UnknownItem
	}
}

func (i item) NewBrowsingData() data.BrowsingData {
	switch i {
	case chromiumKey:
		return nil
	case chromiumPassword:
		return &data.ChromiumPassword{}
	case chromiumCookie:
		return &data.ChromiumCookie{}
	case chromiumBookmark:
		return &data.ChromiumBookmark{}
	case chromiumDownload:
		return &data.ChromiumDownload{}
	case chromiumLocalStorage:
		return nil
	case chromiumCreditCard:
		return &data.ChromiumCreditCard{}
	case chromiumExtension:
		return nil
	case chromiumHistory:
		return &data.ChromiumHistory{}
	case firefoxPassword:
		return &data.FirefoxPassword{}
	case firefoxCookie:
		return &data.FirefoxCookie{}
	case firefoxBookmark:
		return &data.FirefoxBookmark{}
	case firefoxDownload:
		return &data.FirefoxDownload{}
	case firefoxHistory:
		return &data.FirefoxHistory{}
	default:
		return nil
	}
}
