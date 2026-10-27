package chromium

import (
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/types"
)

// sourcePath describes a single candidate location for browser data,
// relative to the profile directory.
type sourcePath struct {
	rel   string // relative path from profileDir, e.g. "Network/Cookies"
	isDir bool   // true for directory targets (LevelDB, Session Storage)
}

// rel stays slash-canonical (e.g. "Network/Cookies"); filepath.Join converts at resolve time, and
// archive reuses it verbatim as a forward-slash zip entry name.
func file(rel string) sourcePath { return sourcePath{rel: rel, isDir: false} }
func dir(rel string) sourcePath  { return sourcePath{rel: rel, isDir: true} }

// chromiumSources defines the standard Chromium file layout.
// Each category maps to one or more candidate paths tried in priority order;
// the first existing path wins.
var chromiumSources = map[category.Category][]sourcePath{
	category.Password:       {file("Login Data"), file(accountLoginData)},
	category.Cookie:         {file("Network/Cookies"), file("Cookies")},
	category.History:        {file("History")},
	category.Download:       {file("History")},
	category.Bookmark:       {file("Bookmarks")},
	category.CreditCard:     {file("Web Data")},
	category.Extension:      {file("Secure Preferences")},
	category.LocalStorage:   {dir("Local Storage/leveldb")},
	category.SessionStorage: {dir("Session Storage")},
}

// sourcesForKind returns the source mapping for a browser kind.
func sourcesForKind(kind types.BrowserKind) map[category.Category][]sourcePath {
	switch kind {
	case types.ChromiumYandex:
		return yandexSources()
	default:
		return chromiumSources
	}
}

// categoryExtractor extracts data for a single category into BrowserData.
// Implementations wrap typed extract functions to provide a uniform dispatch
// interface while preserving the original function signatures.
//
// Use extractorsForKind to register per-Kind overrides. When an extractor
// is present for a category, extractCategory uses it instead of the default
// switch logic, enabling browser-specific parsing (e.g. Opera's opsettings
// for extensions, Yandex's credit card table, QBCI-encrypted bookmarks).
type categoryExtractor interface {
	extract(masterKeys masterkey.Set, path string, data *types.BrowserData) error
}

// passwordExtractor wraps a custom password extract function.
type passwordExtractor struct {
	fn func(masterKeys masterkey.Set, path string) ([]record.Login, error)
}

func (e passwordExtractor) extract(masterKeys masterkey.Set, path string, data *types.BrowserData) error {
	var err error
	data.Passwords, err = e.fn(masterKeys, path)
	return err
}

// extensionExtractor wraps a custom extension extract function.
type extensionExtractor struct {
	fn func(path string) ([]record.Extension, error)
}

func (e extensionExtractor) extract(_ masterkey.Set, path string, data *types.BrowserData) error {
	var err error
	data.Extensions, err = e.fn(path)
	return err
}

// creditCardExtractor wraps a custom credit-card extract function, used by Yandex whose Ya Credit Cards DB stores
// rows as records(guid, public_data, private_data) with JSON blobs rather than Chromium's flat credit_cards table.
type creditCardExtractor struct {
	fn func(masterKeys masterkey.Set, path string) ([]record.CreditCard, error)
}

func (e creditCardExtractor) extract(masterKeys masterkey.Set, path string, data *types.BrowserData) error {
	var err error
	data.CreditCards, err = e.fn(masterKeys, path)
	return err
}

// operaExtractors overrides Extension extraction for Opera,
// which stores settings under "extensions.opsettings".
var operaExtractors = map[category.Category]categoryExtractor{
	category.Extension: extensionExtractor{fn: extractOperaExtensions},
}

// extractorsForKind returns custom category extractors for a browser kind.
// nil means all categories use the default extractCategory switch logic.
func extractorsForKind(kind types.BrowserKind) map[category.Category]categoryExtractor {
	switch kind {
	case types.ChromiumYandex:
		return yandexExtractors
	case types.ChromiumOpera:
		return operaExtractors
	default:
		return nil
	}
}
