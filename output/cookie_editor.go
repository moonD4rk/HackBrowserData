package output

import (
	"encoding/json"
	"io"
)

type cookieEditorFormatter struct{}

func (f *cookieEditorFormatter) ext() string { return "json" }

func (f *cookieEditorFormatter) format(w io.Writer, data any) error {
	rows, ok := data.([]cookieRow)
	if !ok {
		return nil // silently skip non-cookie categories
	}

	entries := make([]cookieEditorEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, cookieEditorEntry{
			Domain:         r.Host,
			ExpirationDate: float64(r.ExpireAt.Unix()),
			HTTPOnly:       r.IsHTTPOnly,
			Name:           r.Name,
			Path:           r.Path,
			Secure:         r.IsSecure,
			Value:          r.Value,
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
