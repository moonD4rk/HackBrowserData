package types

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
}

func TestItem_FileName(t *testing.T) {
	for _, item := range DefaultChromiumTypes {
		assert.Equal(t, item.Filename(), item.filename())
	}
	for _, item := range DefaultFirefoxTypes {
		assert.Equal(t, item.Filename(), item.filename())
	}
	for _, item := range DefaultYandexTypes {
		assert.Equal(t, item.Filename(), item.filename())
	}
}

func TestItem_TempFilename(t *testing.T) {
	asserts := assert.New(t)

	testCases := []struct {
		item     BrowserDataType
		expected string
	}{
		{ChromiumKey, "Local State"},
		{ChromiumPassword, "Login Data"},
		{ChromiumLocalStorage, "Local Storage/leveldb"},
		{FirefoxSessionStorage, "unsupported item"},
		{FirefoxLocalStorage, "webappsstore.sqlite"},
		{YandexPassword, "Ya Passman Data"},
		{YandexCreditCard, "Ya Credit Cards"},
	}

	for _, tc := range testCases {
		expectedPrefix := tc.expected + "_" + strconv.Itoa(int(tc.item)) + ".temp"
		actualPath := tc.item.TempFilename()
		asserts.Contains(actualPath, expectedPrefix, "TempFilename should contain the correct prefix for "+tc.expected)
		asserts.Contains(actualPath, os.TempDir(), "TempFilename should be in the system temp directory for "+tc.expected)
	}
}

func TestItem_IsSensitive(t *testing.T) {
	asserts := assert.New(t)
	testCases := []struct {
		item     BrowserDataType
		expected bool
	}{
		{ChromiumKey, true},
		{ChromiumPassword, true},
		{ChromiumBookmark, false},
	}
	for _, tc := range testCases {
		asserts.Equal(tc.expected, tc.item.IsSensitive(), fmt.Sprintf("IsSensitive for %v should be %v", tc.item, tc.expected))
	}
}

func TestFilterSensitiveItems(t *testing.T) {
	asserts := assert.New(t)
	testCases := []struct {
		items    []BrowserDataType
		expected int
	}{
		{[]BrowserDataType{ChromiumKey, ChromiumBookmark, ChromiumPassword}, 2},
		{[]BrowserDataType{ChromiumBookmark, ChromiumHistory}, 0},
	}

	for _, tc := range testCases {
		filteredItems := FilterSensitiveItems(tc.items)
		asserts.Len(filteredItems, tc.expected, "FilterSensitiveItems should return the correct number of sensitive items")
		for _, item := range filteredItems {
			asserts.True(item.IsSensitive(), "Filtered items should be sensitive")
		}
	}
}

func (i BrowserDataType) filename() string {
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
	case ChromiumSessionStorage:
		return fileChromiumSessionStorage
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
		return UnsupportedItem
	}
}
