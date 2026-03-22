package browserdata

import "github.com/moond4rk/hackbrowserdata/types"

// Data holds all extracted data from one browser profile.
// Each field is a slice that may be nil (not supported) or empty (no data found).
// This struct will replace the current BrowserData once the refactoring is complete.
type Data struct {
	Passwords      []types.LoginEntry      `json:"passwords,omitempty"`
	Cookies        []types.CookieEntry     `json:"cookies,omitempty"`
	Bookmarks      []types.BookmarkEntry   `json:"bookmarks,omitempty"`
	Histories      []types.HistoryEntry    `json:"histories,omitempty"`
	Downloads      []types.DownloadEntry   `json:"downloads,omitempty"`
	CreditCards    []types.CreditCardEntry `json:"credit_cards,omitempty"`
	Extensions     []types.ExtensionEntry  `json:"extensions,omitempty"`
	LocalStorage   []types.StorageEntry    `json:"local_storage,omitempty"`
	SessionStorage []types.StorageEntry    `json:"session_storage,omitempty"`
}
