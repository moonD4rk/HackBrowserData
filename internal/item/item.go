package item

import (
	"hack-browser-data/internal/browser/data"
)

type Item int

const (
	ChromiumKey Item = iota
	ChromiumPassword
	ChromiumCookie
	ChromiumBookmark
	ChromiumHistory
	ChromiumDownload
	ChromiumCreditCard
	ChromiumLocalStorage
	ChromiumExtension

	YandexPassword
	YandexCreditCard

	FirefoxKey4
	FirefoxPassword
	FirefoxCookie
	FirefoxBookmark
	FirefoxHistory
	FirefoxDownload
	FirefoxCreditCard
	FirefoxLocalStorage
	FirefoxExtension
)

var DefaultFirefox = []Item{
	FirefoxKey4,
	FirefoxPassword,
	FirefoxCookie,
	FirefoxBookmark,
	FirefoxHistory,
	FirefoxDownload,
	FirefoxCreditCard,
	FirefoxLocalStorage,
	FirefoxExtension,
}

var DefaultYandex = []Item{
	ChromiumKey,
	ChromiumCookie,
	ChromiumBookmark,
	ChromiumHistory,
	ChromiumDownload,
	ChromiumLocalStorage,
	ChromiumExtension,
	YandexPassword,
	YandexCreditCard,
}

var DefaultChromium = []Item{
	ChromiumKey,
	ChromiumPassword,
	ChromiumCookie,
	ChromiumBookmark,
	ChromiumHistory,
	ChromiumDownload,
	ChromiumCreditCard,
	ChromiumLocalStorage,
	ChromiumExtension,
}

func (i Item) FileName() string {
	switch i {
	case ChromiumKey:
		return fileChromiumKey
	case ChromiumPassword:
		return fileChromiumPassword
	case ChromiumCookie:
		return fileChromiumCookie
	case ChromiumBookmark:
		return fileChromiumBookmark
	case ChromiumDownload:
		return fileChromiumDownload
	case ChromiumLocalStorage:
		return fileChromiumLocalStorage
	case ChromiumCreditCard:
		return fileChromiumCredit
	case ChromiumExtension:
		return UnknownItem
	case ChromiumHistory:
		return fileChromiumHistory
	case YandexPassword:
		return fileYandexPassword
	case YandexCreditCard:
		return fileYandexCredit
	case FirefoxKey4:
		return fileFirefoxKey4
	case FirefoxPassword:
		return fileFirefoxPassword
	case FirefoxCookie:
		return fileFirefoxCookie
	case FirefoxBookmark:
		return fileFirefoxData
	case FirefoxDownload:
		return fileFirefoxData
	case FirefoxLocalStorage:
		return UnsupportedItem
	case FirefoxCreditCard:
		return UnsupportedItem
	case FirefoxHistory:
		return fileFirefoxData
	case FirefoxExtension:
		return UnsupportedItem
	default:
		return UnknownItem
	}
}

func (i Item) String() string {
	switch i {
	case ChromiumKey:
		return "chromiumKey"
	case ChromiumPassword:
		return "password"
	case ChromiumCookie:
		return "cookie"
	case ChromiumBookmark:
		return "bookmark"
	case ChromiumDownload:
		return "download"
	case ChromiumLocalStorage:
		return "localStorage"
	case ChromiumCreditCard:
		return "creditCard"
	case ChromiumExtension:
		return UnsupportedItem
	case ChromiumHistory:
		return "history"
	case YandexPassword:
		return "yandexPassword"
	case YandexCreditCard:
		return "yandexCreditCard"
	case FirefoxKey4:
		return "firefoxKey4"
	case FirefoxPassword:
		return "firefoxPassword"
	case FirefoxCookie:
		return "firefoxCookie"
	case FirefoxBookmark:
		return "firefoxBookmark"
	case FirefoxDownload:
		return "firefoxDownload"
	case FirefoxHistory:
		return "firefoxHistory"
	case FirefoxLocalStorage:
		return UnsupportedItem
	case FirefoxCreditCard:
		return UnsupportedItem
	case FirefoxExtension:
		return UnsupportedItem
	default:
		return UnknownItem
	}
}

func (i Item) NewBrowsingData() data.BrowsingData {
	switch i {
	case ChromiumKey:
		return nil
	case ChromiumPassword:
		return &data.ChromiumPassword{}
	case ChromiumCookie:
		return &data.ChromiumCookie{}
	case ChromiumBookmark:
		return &data.ChromiumBookmark{}
	case ChromiumDownload:
		return &data.ChromiumDownload{}
	case ChromiumLocalStorage:
		return nil
	case ChromiumCreditCard:
		return &data.ChromiumCreditCard{}
	case ChromiumExtension:
		return nil
	case ChromiumHistory:
		return &data.ChromiumHistory{}
	case YandexPassword:
		return &data.ChromiumPassword{}
	case YandexCreditCard:
		return &data.ChromiumCreditCard{}
	case FirefoxPassword:
		return &data.FirefoxPassword{}
	case FirefoxCookie:
		return &data.FirefoxCookie{}
	case FirefoxBookmark:
		return &data.FirefoxBookmark{}
	case FirefoxDownload:
		return &data.FirefoxDownload{}
	case FirefoxHistory:
		return &data.FirefoxHistory{}
	default:
		return nil
	}
}
