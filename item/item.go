package item

import (
	"fmt"
	"os"
	"path/filepath"
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
	ChromiumSessionStorage
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
	FirefoxSessionStorage
	FirefoxExtension
)

var itemFileNames = map[Item]string{
	ChromiumKey:            fileChromiumKey,
	ChromiumPassword:       fileChromiumPassword,
	ChromiumCookie:         fileChromiumCookie,
	ChromiumBookmark:       fileChromiumBookmark,
	ChromiumDownload:       fileChromiumDownload,
	ChromiumLocalStorage:   fileChromiumLocalStorage,
	ChromiumSessionStorage: fileChromiumSessionStorage,
	ChromiumCreditCard:     fileChromiumCredit,
	ChromiumExtension:      fileChromiumExtension,
	ChromiumHistory:        fileChromiumHistory,
	YandexPassword:         fileYandexPassword,
	YandexCreditCard:       fileYandexCredit,
	FirefoxKey4:            fileFirefoxKey4,
	FirefoxPassword:        fileFirefoxPassword,
	FirefoxCookie:          fileFirefoxCookie,
	FirefoxBookmark:        fileFirefoxData,
	FirefoxDownload:        fileFirefoxData,
	FirefoxLocalStorage:    fileFirefoxLocalStorage,
	FirefoxHistory:         fileFirefoxData,
	FirefoxExtension:       fileFirefoxExtension,
	FirefoxSessionStorage:  UnsupportedItem,
	FirefoxCreditCard:      UnsupportedItem,
}

func (i Item) Filename() string {
	if fileName, ok := itemFileNames[i]; ok {
		return fileName
	}
	return UnsupportedItem
}

const tempSuffix = "temp"

// TempFilename returns the temp filename for the item with suffix
// eg: chromiumKey_0.temp
func (i Item) TempFilename() string {
	tempFile := fmt.Sprintf("%s_%d.%s", i.Filename(), i, tempSuffix)
	return filepath.Join(os.TempDir(), tempFile)
}

// IsSensitive returns whether the item is sensitive data
// password, cookie, credit card, master key is unlimited
func (i Item) IsSensitive() bool {
	switch i {
	case ChromiumKey, ChromiumCookie, ChromiumPassword, ChromiumCreditCard,
		FirefoxKey4, FirefoxPassword, FirefoxCookie, FirefoxCreditCard,
		YandexPassword, YandexCreditCard:
		return true
	default:
		return false
	}
}

// FilterSensitiveItems returns the sensitive items
func FilterSensitiveItems(items []Item) []Item {
	var filtered []Item
	for _, item := range items {
		if item.IsSensitive() {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// DefaultFirefox returns the default items for the firefox browser
var DefaultFirefox = []Item{
	FirefoxKey4,
	FirefoxPassword,
	FirefoxCookie,
	FirefoxBookmark,
	FirefoxHistory,
	FirefoxDownload,
	FirefoxCreditCard,
	FirefoxLocalStorage,
	FirefoxSessionStorage,
	FirefoxExtension,
}

// DefaultYandex returns the default items for the yandex browser
var DefaultYandex = []Item{
	ChromiumKey,
	ChromiumCookie,
	ChromiumBookmark,
	ChromiumHistory,
	ChromiumDownload,
	ChromiumExtension,
	YandexPassword,
	ChromiumLocalStorage,
	ChromiumSessionStorage,
	YandexCreditCard,
}

// DefaultChromium returns the default items for the chromium browser
var DefaultChromium = []Item{
	ChromiumKey,
	ChromiumPassword,
	ChromiumCookie,
	ChromiumBookmark,
	ChromiumHistory,
	ChromiumDownload,
	ChromiumCreditCard,
	ChromiumLocalStorage,
	ChromiumSessionStorage,
	ChromiumExtension,
}

// item's default filename
const (
	fileChromiumKey            = "Local State"
	fileChromiumCredit         = "Web Data"
	fileChromiumPassword       = "Login Data"
	fileChromiumHistory        = "History"
	fileChromiumDownload       = "History"
	fileChromiumCookie         = "Cookies"
	fileChromiumBookmark       = "Bookmarks"
	fileChromiumLocalStorage   = "Local Storage/leveldb"
	fileChromiumSessionStorage = "Session Storage"
	fileChromiumExtension      = "Secure Preferences" // TODO: add more extension files and folders, eg: Preferences

	fileYandexPassword = "Ya Passman Data"
	fileYandexCredit   = "Ya Credit Cards"

	fileFirefoxKey4         = "key4.db"
	fileFirefoxCookie       = "cookies.sqlite"
	fileFirefoxPassword     = "logins.json"
	fileFirefoxData         = "places.sqlite"
	fileFirefoxLocalStorage = "webappsstore.sqlite"
	fileFirefoxExtension    = "extensions.json"
)

const (
	UnknownItem     = "unknown item"
	UnsupportedItem = "unsupported item"
)
