package item

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
		return fileChromiumExtension
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
		return fileFirefoxLocalStorage
	case FirefoxHistory:
		return fileFirefoxData
	case FirefoxExtension:
		return fileFirefoxExtension
	case FirefoxCreditCard:
		return UnsupportedItem
	default:
		return UnknownItem
	}
}

func (i Item) String() string {
	switch i {
	case ChromiumKey:
		return TempChromiumKey
	case ChromiumPassword:
		return TempChromiumPassword
	case ChromiumCookie:
		return TempChromiumCookie
	case ChromiumBookmark:
		return TempChromiumBookmark
	case ChromiumDownload:
		return TempChromiumDownload
	case ChromiumLocalStorage:
		return TempChromiumLocalStorage
	case ChromiumCreditCard:
		return TempChromiumCreditCard
	case ChromiumExtension:
		return TempChromiumExtension
	case ChromiumHistory:
		return TempChromiumHistory
	case YandexPassword:
		return TempYandexPassword
	case YandexCreditCard:
		return TempYandexCreditCard
	case FirefoxKey4:
		return TempFirefoxKey4
	case FirefoxPassword:
		return TempFirefoxPassword
	case FirefoxCookie:
		return TempFirefoxCookie
	case FirefoxBookmark:
		return TempFirefoxBookmark
	case FirefoxDownload:
		return TempFirefoxDownload
	case FirefoxHistory:
		return TempFirefoxHistory
	case FirefoxLocalStorage:
		return TempFirefoxLocalStorage
	case FirefoxCreditCard:
		return UnsupportedItem
	case FirefoxExtension:
		return TempFirefoxExtension
	default:
		return UnknownItem
	}
}

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
	ChromiumExtension,
	YandexPassword,
	ChromiumLocalStorage,
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
