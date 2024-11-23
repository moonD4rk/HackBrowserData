package types2

import (
	"path/filepath"
)

type DataType int

const (
	MasterKey DataType = iota
	Password
	Cookie
	Bookmark
	History
	Download
	CreditCard
	LocalStorage
	SessionStorage
	Extension
)

func (d DataType) String() string {
	switch d {
	case MasterKey:
		return "MasterKey"
	case Password:
		return "Password"
	case Cookie:
		return "Cookie"
	case Bookmark:
		return "Bookmark"
	case History:
		return "History"
	case Download:
		return "Download"
	case CreditCard:
		return "CreditCard"
	case LocalStorage:
		return "LocalStorage"
	case SessionStorage:
		return "SessionStorage"
	case Extension:
		return "Extension"
	default:
		return "Unknown"
	}
}

func (d DataType) Info(browserType BrowserType) DataTypeInfo {
	if browserData, ok := typesDataMap[browserType]; ok {
		if info, ok := browserData[d]; ok {
			return info
		}
	}
	return DataTypeInfo{}
}

func (d DataType) Filename(browserType BrowserType) string {
	return d.Info(browserType).Filename()
}

func (d DataType) IsDir(browserType BrowserType) bool {
	return d.Info(browserType).IsDir()
}

var AllDataTypes = []DataType{
	MasterKey,
	Password,
	Cookie,
	Bookmark,
	History,
	Download,
	CreditCard,
	LocalStorage,
	SessionStorage,
	Extension,
}

type BrowserType int

const (
	ChromiumType BrowserType = iota
	FirefoxType
	YandexType
)

func (b BrowserType) String() string {
	switch b {
	case ChromiumType:
		return "ChromiumType"
	case FirefoxType:
		return "FirefoxType"
	case YandexType:
		return "YandexType"
	default:
		return "Unknown"
	}
}

var typesDataMap = map[BrowserType]map[DataType]DataTypeInfo{
	ChromiumType: chromiumDataMap,
	FirefoxType:  firefoxDataMap,
	YandexType:   yandexDataMap,
}

var chromiumDataMap = map[DataType]DataTypeInfo{
	MasterKey:      {filename: "Local State"},
	Password:       {filename: "Login Data"},
	Cookie:         {filename: "Cookies"},
	Bookmark:       {filename: "Bookmarks"},
	History:        {filename: "History"},
	Download:       {filename: "History"},
	CreditCard:     {filename: "Web Data"},
	LocalStorage:   {filename: filepath.Join("Local Storage", "leveldb"), isDir: true},
	SessionStorage: {filename: "Session Storage", isDir: true},
	Extension:      {filename: "Secure Preferences", alternateNames: []string{"Preferences"}},
}

var firefoxDataMap = map[DataType]DataTypeInfo{
	MasterKey: {filename: "key4.db"},
	Password:  {filename: "logins.json"},
	Cookie:    {filename: "cookies.sqlite"},
	Bookmark:  {filename: "places.sqlite"},
	History:   {filename: "places.sqlite"},
	Download:  {filename: "places.sqlite"},
	// CreditCard:     {"logins.json"},
	LocalStorage:   {filename: "webappsstore.sqlite"},
	SessionStorage: {filename: "sessionstore.jsonlz4"},
	Extension:      {filename: "extensions.json"},
}

var yandexDataMap = map[DataType]DataTypeInfo{
	MasterKey:      {filename: "Local State"},
	Password:       {filename: "Login Data"},
	Cookie:         {filename: "Cookies"},
	Bookmark:       {filename: "Bookmarks"},
	History:        {filename: "History"},
	Download:       {filename: "History"},
	CreditCard:     {filename: "Web Data"},
	LocalStorage:   {filename: filepath.Join("Local Storage", "leveldb"), isDir: true},
	SessionStorage: {filename: "Session Storage", isDir: true},
	Extension:      {filename: "Secure Preferences", alternateNames: []string{"Preferences"}},
}
