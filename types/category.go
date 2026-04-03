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
	KindChromium       BrowserKind = iota
	KindChromiumYandex             // Chromium variant with different file names and extract logic
	KindChromiumOpera              // Opera: extensions in "opsettings" key, data in Roaming
	KindFirefox
)

// BrowserConfig holds the declarative configuration for a browser installation.
type BrowserConfig struct {
	Key              string      // lookup key: "chrome", "edge", "firefox"
	Name             string      // display name: "Chrome", "Edge", "Firefox"
	Kind             BrowserKind // engine type
	Storage          string      // keychain/GNOME label (macOS/Linux); unused on Windows
	KeychainPassword string      // macOS login password for KeychainPasswordRetriever; ignored on Windows/Linux
	UserDataDir      string      // base browser directory
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

// CategoryData holds one category's data with its metadata,
// used by BrowserData.Each() for generic iteration.
type CategoryData struct {
	Category Category
	Data     interface{} // typed slice ([]LoginEntry, []CookieEntry, etc.)
	Len      int
}

// Each iterates over all non-empty categories in BrowserData.
func (d *BrowserData) Each(fn func(CategoryData)) {
	items := []CategoryData{
		{Password, d.Passwords, len(d.Passwords)},
		{Cookie, d.Cookies, len(d.Cookies)},
		{History, d.Histories, len(d.Histories)},
		{Download, d.Downloads, len(d.Downloads)},
		{Bookmark, d.Bookmarks, len(d.Bookmarks)},
		{CreditCard, d.CreditCards, len(d.CreditCards)},
		{Extension, d.Extensions, len(d.Extensions)},
		{LocalStorage, d.LocalStorage, len(d.LocalStorage)},
		{SessionStorage, d.SessionStorage, len(d.SessionStorage)},
	}
	for _, item := range items {
		if item.Len > 0 {
			fn(item)
		}
	}
}
