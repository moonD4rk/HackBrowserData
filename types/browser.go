package types

import "github.com/moond4rk/hackbrowserdata/internal/record"

// BrowserKind identifies the browser engine type.
type BrowserKind int

const (
	Chromium       BrowserKind = iota
	ChromiumYandex             // Chromium variant with different file names and extract logic
	ChromiumOpera              // Opera: extensions in "opsettings" key, data in Roaming
	Firefox
	Safari
)

// String returns the canonical lowercase name of the engine kind; the chromium-family values are
// also the stable wire form carried in a keys dump, so don't change them lightly.
func (k BrowserKind) String() string {
	switch k {
	case Chromium:
		return "chromium"
	case ChromiumYandex:
		return "chromium-yandex"
	case ChromiumOpera:
		return "chromium-opera"
	case Firefox:
		return "firefox"
	case Safari:
		return "safari"
	default:
		return "unknown"
	}
}

// BrowserConfig holds the declarative configuration for a browser installation.
type BrowserConfig struct {
	Key           string      // Def.ID in internal/browser; the Windows injector looks the ABE target up by it
	Name          string      // display name
	Kind          BrowserKind // engine type
	KeychainLabel string      // macOS Keychain account / Linux D-Bus Secret Service label; "" = none
	WindowsABE    bool        // enable Windows App-Bound Encryption v20 (reflective injection)
	UserDataDir   string      // base browser directory
}

// BrowserData holds all extracted browser data with typed slices.
type BrowserData struct {
	Passwords      []record.Login
	Cookies        []record.Cookie
	Histories      []record.History
	Downloads      []record.Download
	Bookmarks      []record.Bookmark
	CreditCards    []record.CreditCard
	Extensions     []record.Extension
	LocalStorage   []record.LocalStorage
	SessionStorage []record.SessionStorage
}
