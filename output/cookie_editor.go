package output

import (
	"encoding/json"
	"io"

	"github.com/moond4rk/hackbrowserdata/types"
)

// CookieEditorFormatter writes cookies in CookieEditor browser extension format.
// Non-cookie categories are silently skipped.
type CookieEditorFormatter struct{}

func (f *CookieEditorFormatter) Ext() string { return "json" }

func (f *CookieEditorFormatter) Format(w io.Writer, cd types.CategoryData, _, _ string) error {
	cookies, ok := cd.Data.([]types.CookieEntry)
	if !ok {
		return nil // silently skip non-cookie categories
	}

	entries := make([]cookieEditorEntry, 0, len(cookies))
	for _, c := range cookies {
		entries = append(entries, cookieEditorEntry{
			Domain:         c.Host,
			ExpirationDate: float64(c.ExpireAt.Unix()),
			HTTPOnly:       c.IsHTTPOnly,
			Name:           c.Name,
			Path:           c.Path,
			Secure:         c.IsSecure,
			Value:          c.Value,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(entries)
}

// cookieEditorEntry matches the CookieEditor browser extension's import format.
type cookieEditorEntry struct {
	Domain         string  `json:"domain"`
	ExpirationDate float64 `json:"expirationDate"`
	HTTPOnly       bool    `json:"httpOnly"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	Secure         bool    `json:"secure"`
	Value          string  `json:"value"`
}
