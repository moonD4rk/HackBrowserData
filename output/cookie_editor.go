package output

import (
	"encoding/json"
	"io"

	"github.com/moond4rk/hackbrowserdata/types"
)

type cookieEditorFormatter struct{}

func (f *cookieEditorFormatter) ext() string { return "json" }

func (f *cookieEditorFormatter) format(w io.Writer, rows []row) error {
	entries := make([]cookieEditorEntry, 0, len(rows))
	for _, r := range rows {
		c, ok := r.entry.(types.CookieEntry)
		if !ok {
			return nil // not cookies, skip
		}
		var expDate float64
		if !c.ExpireAt.IsZero() {
			expDate = float64(c.ExpireAt.Unix())
		}
		entries = append(entries, cookieEditorEntry{
			Domain:         c.Host,
			ExpirationDate: expDate,
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
