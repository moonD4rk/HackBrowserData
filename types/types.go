package types

import (
	"fmt"
	"os"
	"path/filepath"
)

type DataType int

const (
	ChromiumKey DataType = iota
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

var itemFileNames = map[DataType]string{
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

func (i DataType) String() string {
	switch i {
	case ChromiumKey:
		return "ChromiumKey"
	case ChromiumPassword:
		return "ChromiumPassword"
	case ChromiumCookie:
		return "ChromiumCookie"
	case ChromiumBookmark:
		return "ChromiumBookmark"
	case ChromiumHistory:
		return "ChromiumHistory"
	case ChromiumDownload:
		return "ChromiumDownload"
	case ChromiumCreditCard:
		return "ChromiumCreditCard"
	case ChromiumLocalStorage:
		return "ChromiumLocalStorage"
	case ChromiumSessionStorage:
		return "ChromiumSessionStorage"
	case ChromiumExtension:
		return "ChromiumExtension"
	case YandexPassword:
		return "YandexPassword"
	case YandexCreditCard:
		return "YandexCreditCard"
	case FirefoxKey4:
		return "FirefoxKey4"
	case FirefoxPassword:
		return "FirefoxPassword"
	case FirefoxCookie:
		return "FirefoxCookie"
	case FirefoxBookmark:
		return "FirefoxBookmark"
	case FirefoxHistory:
		return "FirefoxHistory"
	case FirefoxDownload:
		return "FirefoxDownload"
	case FirefoxCreditCard:
		return "FirefoxCreditCard"
	case FirefoxLocalStorage:
		return "FirefoxLocalStorage"
	case FirefoxSessionStorage:
		return "FirefoxSessionStorage"
	case FirefoxExtension:
		return "FirefoxExtension"
	default:
		return "UnsupportedItem"
	}
}

// Filename returns the filename for the item, defined by browser
// chromium local storage is a folder, so it returns the file name of the folder
func (i DataType) Filename() string {
	if fileName, ok := itemFileNames[i]; ok {
		return fileName
	}
	return UnsupportedItem
}

// TempFilename returns the temp filename for the item with suffix
// eg: chromiumKey_0.temp
func (i DataType) TempFilename() string {
	const tempSuffix = "temp"
	tempFile := fmt.Sprintf("%s_%d.%s", i.Filename(), i, tempSuffix)
	return filepath.Join(os.TempDir(), tempFile)
}

// IsSensitive returns whether the item is sensitive data
// password, cookie, credit card, master key is unlimited
func (i DataType) IsSensitive() bool {
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
func FilterSensitiveItems(items []DataType) []DataType {
	var filtered []DataType
	for _, item := range items {
		if item.IsSensitive() {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// DefaultFirefoxTypes returns the default items for the firefox browser
var DefaultFirefoxTypes = []DataType{
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
var DefaultYandexTypes = []DataType{
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
var DefaultChromiumTypes = []DataType{
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

	UnsupportedItem = "unsupported item"
)
