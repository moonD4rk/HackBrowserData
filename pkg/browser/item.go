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
	chromiumCreditcard
	chromiumLocalStorage
	chromiumExtension

	firefoxKey4
	firefoxPassword
	firefoxCookie
	firefoxBookmark
	firefoxHistory
	firefoxDownload
	firefoxCreditcard
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
	case chromiumCreditcard:
		return consts.ChromiumCredit
	case chromiumExtension:
		return "unsupport item"
	case chromiumHistory:
		return consts.ChromiumHistory
	case firefoxPassword:
		return consts.FirefoxLogin
	case firefoxCookie:
		return consts.FirefoxData
	default:
		return "unknown item"
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
	case chromiumCreditcard:
		return consts.ChromiumCreditFilename
	case chromiumExtension:
		return "unsupport item"
	case chromiumHistory:
		return consts.ChromiumHistoryFilename
	case firefoxPassword:
		return consts.FirefoxLoginFilename
	case firefoxCookie:
		return consts.FirefoxDataFilename
	default:
		return "unknown item"
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
	case chromiumCreditcard:
		return &data.ChromiumCreditCard{}
	case chromiumExtension:
		return nil
	case chromiumHistory:
		return &data.ChromiumHistory{}
	case firefoxPassword:
		return nil
	case firefoxCookie:
		return nil
	default:
		return nil
	}
}
