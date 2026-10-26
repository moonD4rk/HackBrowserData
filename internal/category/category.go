package category

import "fmt"

// Numbering starts at 1, so the zero Category means "none".
type Category int

const (
	Password Category = iota + 1
	Cookie
	Bookmark
	History
	Download
	CreditCard
	Extension
	LocalStorage
	SessionStorage
)

// String's values are the -c values and the output file names, so they cannot change.
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

func All() []Category {
	return []Category{Password, Cookie, Bookmark, History, Download, CreditCard, Extension, LocalStorage, SessionStorage}
}

func Parse(s string) (Category, error) {
	for _, c := range All() {
		if c.String() == s {
			return c, nil
		}
	}
	return 0, fmt.Errorf("unknown category: %q", s)
}
