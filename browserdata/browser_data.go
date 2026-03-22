package browserdata

import "github.com/moond4rk/hackbrowserdata/types"

// Data holds all extracted data from one browser profile.
// Each field is a slice that may be nil (not supported) or empty (no data found).
// This struct will replace the current BrowserData once the refactoring is complete.
type Data struct {
	Passwords      []types.LoginEntry
	Cookies        []types.CookieEntry
	Bookmarks      []types.BookmarkEntry
	Histories      []types.HistoryEntry
	Downloads      []types.DownloadEntry
	CreditCards    []types.CreditCardEntry
	Extensions     []types.ExtensionEntry
	LocalStorage   []types.StorageEntry
	SessionStorage []types.StorageEntry
}
