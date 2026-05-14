package types

// Category represents a kind of browser data.
// It is browser-agnostic — a password is a password regardless of which browser it came from.
type Category int

const (
	Password Category = iota
	Cookie
	Bookmark
	History
	Download
	CreditCard
	Extension
	LocalStorage
	SessionStorage
)

// AllCategories returns all supported data categories.
var AllCategories = []Category{
	Password, Cookie, Bookmark, History, Download,
	CreditCard, Extension, LocalStorage, SessionStorage,
}

// String returns the human-readable name of the category.
func (c Category) String() string {
	switch c {
	case Password:
		return "password"
	case Cookie:
		return "cookie"
	case Bookmark:
		return "bookmark"
	case History:
		return "history"
	case Download:
		return "download"
	case CreditCard:
		return "creditcard"
	case Extension:
		return "extension"
	case LocalStorage:
		return "localstorage"
	case SessionStorage:
		return "sessionstorage"
	default:
		return "unknown"
	}
}

// IsSensitive returns whether the category contains sensitive data
// that requires explicit opt-in to export.
func (c Category) IsSensitive() bool {
	switch c {
	case Password, Cookie, CreditCard:
		return true
	default:
		return false
	}
}

// NonSensitiveCategories returns categories that are safe to export by default.
func NonSensitiveCategories() []Category {
	var cats []Category
	for _, c := range AllCategories {
		if !c.IsSensitive() {
			cats = append(cats, c)
		}
	}
	return cats
}

// BrowserKind identifies the browser engine type.
type BrowserKind int

const (
	Chromium       BrowserKind = iota
	ChromiumYandex             // Chromium variant with different file names and extract logic
	ChromiumOpera              // Opera: extensions in "opsettings" key, data in Roaming
	Firefox
	Safari
)

// BrowserConfig holds the declarative configuration for a browser installation.
type BrowserConfig struct {
	Key           string      // lookup key; doubles as the Windows ABE / winutil.Table key when WindowsABE is true
	Name          string      // display name
	Kind          BrowserKind // engine type
	KeychainLabel string      // macOS Keychain account / Linux D-Bus Secret Service label; "" = none
	WindowsABE    bool        // enable Windows App-Bound Encryption v20 (reflective injection)
	UserDataDir   string      // base browser directory
}

// BrowserData holds all extracted browser data with typed slices.
type BrowserData struct {
	Passwords      []LoginEntry
	Cookies        []CookieEntry
	Histories      []HistoryEntry
	Downloads      []DownloadEntry
	Bookmarks      []BookmarkEntry
	CreditCards    []CreditCardEntry
	Extensions     []ExtensionEntry
	LocalStorage   []StorageEntry
	SessionStorage []StorageEntry
}
