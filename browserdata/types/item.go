package types

import (
	"fmt"
	"os"
	"path/filepath"
)

type BrowserDataType int

const (
	ChromiumKey BrowserDataType = iota
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

var itemFileNames = map[BrowserDataType]string{
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

// Filename returns the filename for the item, defined by browser
// chromium local storage is a folder, so it returns the file name of the folder
func (i BrowserDataType) Filename() string {
	if fileName, ok := itemFileNames[i]; ok {
		return fileName
	}
	return UnsupportedItem
}

// TempFilename returns the temp filename for the item with suffix
// eg: chromiumKey_0.temp
func (i BrowserDataType) TempFilename() string {
	const tempSuffix = "temp"
	tempFile := fmt.Sprintf("%s_%d.%s", i.Filename(), i, tempSuffix)
	return filepath.Join(os.TempDir(), tempFile)
}

// IsSensitive returns whether the item is sensitive data
// password, cookie, credit card, master key is unlimited
func (i BrowserDataType) IsSensitive() bool {
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
func FilterSensitiveItems(items []BrowserDataType) []BrowserDataType {
	var filtered []BrowserDataType
	for _, item := range items {
		if item.IsSensitive() {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// DefaultFirefoxTypes returns the default items for the firefox browser
var DefaultFirefoxTypes = []BrowserDataType{
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

// DefaultYandexTypes returns the default items for the yandex browser
var DefaultYandexTypes = []BrowserDataType{
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

// DefaultChromiumTypes returns the default items for the chromium browser
var DefaultChromiumTypes = []BrowserDataType{
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
	UnsupportedItem = "unsupported item"
)
