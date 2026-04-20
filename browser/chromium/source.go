package chromium

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/types"
)

// sourcePath describes a single candidate location for browser data,
// relative to the profile directory.
type sourcePath struct {
	rel   string // relative path from profileDir, e.g. "Network/Cookies"
	isDir bool   // true for directory targets (LevelDB, Session Storage)
}

func file(rel string) sourcePath { return sourcePath{rel: filepath.FromSlash(rel), isDir: false} }
func dir(rel string) sourcePath  { return sourcePath{rel: filepath.FromSlash(rel), isDir: true} }

// chromiumSources defines the standard Chromium file layout.
// Each category maps to one or more candidate paths tried in priority order;
// the first existing path wins.
var chromiumSources = map[types.Category][]sourcePath{
	types.Password:       {file("Login Data")},
	types.Cookie:         {file("Network/Cookies"), file("Cookies")},
	types.History:        {file("History")},
	types.Download:       {file("History")},
	types.Bookmark:       {file("Bookmarks")},
	types.CreditCard:     {file("Web Data")},
	types.Extension:      {file("Secure Preferences")},
	types.LocalStorage:   {dir("Local Storage/leveldb")},
	types.SessionStorage: {dir("Session Storage")},
}

// yandexSourceOverrides contains only the entries that differ from chromiumSources.
var yandexSourceOverrides = map[types.Category][]sourcePath{
	types.Password:   {file("Ya Passman Data")},
	types.CreditCard: {file("Ya Credit Cards")},
}

// yandexSources returns chromiumSources with Yandex-specific overrides applied.
func yandexSources() map[types.Category][]sourcePath {
	sources := make(map[types.Category][]sourcePath, len(chromiumSources))
	for k, v := range chromiumSources {
		sources[k] = v
	}
	for k, v := range yandexSourceOverrides {
		sources[k] = v
	}
	return sources
}

// sourcesForKind returns the source mapping for a browser kind.
func sourcesForKind(kind types.BrowserKind) map[types.Category][]sourcePath {
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
	extract(keys keyretriever.MasterKeys, path string, data *types.BrowserData) error
}

// passwordExtractor wraps a custom password extract function.
type passwordExtractor struct {
	fn func(keys keyretriever.MasterKeys, path string) ([]types.LoginEntry, error)
}

func (e passwordExtractor) extract(keys keyretriever.MasterKeys, path string, data *types.BrowserData) error {
	var err error
	data.Passwords, err = e.fn(keys, path)
	return err
}

// extensionExtractor wraps a custom extension extract function.
type extensionExtractor struct {
	fn func(path string) ([]types.ExtensionEntry, error)
}

func (e extensionExtractor) extract(_ keyretriever.MasterKeys, path string, data *types.BrowserData) error {
	var err error
	data.Extensions, err = e.fn(path)
	return err
}

// yandexExtractors overrides Password extraction for Yandex,
// which uses action_url instead of origin_url.
var yandexExtractors = map[types.Category]categoryExtractor{
	types.Password: passwordExtractor{fn: extractYandexPasswords},
}

// operaExtractors overrides Extension extraction for Opera,
// which stores settings under "extensions.opsettings".
var operaExtractors = map[types.Category]categoryExtractor{
	types.Extension: extensionExtractor{fn: extractOperaExtensions},
}

// extractorsForKind returns custom category extractors for a browser kind.
// nil means all categories use the default extractCategory switch logic.
func extractorsForKind(kind types.BrowserKind) map[types.Category]categoryExtractor {
	switch kind {
	case types.ChromiumYandex:
		return yandexExtractors
	case types.ChromiumOpera:
		return operaExtractors
	default:
		return nil
	}
}
