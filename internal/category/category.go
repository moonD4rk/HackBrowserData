package category

import "fmt"

// Category numbering starts at 1, so the zero Category means "none".
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

// String gives the -c values and the output file names, so its values cannot change.
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

func Names() []string {
	all := All()
	names := make([]string, len(all))
	for i, dataCategory := range all {
		names[i] = dataCategory.String()
	}
	return names
}

func Parse(value string) (Category, error) {
	for _, dataCategory := range All() {
		if dataCategory.String() == value {
			return dataCategory, nil
		}
	}
	return 0, fmt.Errorf("unknown category: %q", value)
}
