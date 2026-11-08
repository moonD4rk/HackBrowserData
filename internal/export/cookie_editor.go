package export

import (
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var sameSiteNone = "no_restriction"

func toCookieEditor(row record.Record) any {
	cookie, ok := row.(*record.Cookie)
	if !ok {
		return row
	}
	var expDate float64
	if !cookie.ExpireAt.IsZero() {
		expDate = float64(cookie.ExpireAt.Unix())
	}
	sameSite := &cookie.SameSite
	switch cookie.SameSite {
	case "none":
		sameSite = &sameSiteNone
	case "", "unspecified":
		sameSite = nil
	}
	return cookieEditorEntry{
		Domain:         cookie.Host,
		ExpirationDate: expDate,
		HTTPOnly:       cookie.IsHTTPOnly,
		Name:           cookie.Name,
		Path:           cookie.Path,
		Secure:         cookie.IsSecure,
		Value:          cookie.Value,
		SameSite:       sameSite,
		Session:        expDate == 0.,
		HostOnly:       !strings.HasPrefix(cookie.Host, "."),
	}
}

// cookieEditorEntry matches the CookieEditor browser extension's import format.
type cookieEditorEntry struct {
	Domain         string  `json:"domain"`
	ExpirationDate float64 `json:"expirationDate,omitempty"`
	HTTPOnly       bool    `json:"httpOnly"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	Secure         bool    `json:"secure"`
	Value          string  `json:"value"`
	SameSite       *string `json:"sameSite"`
	Session        bool    `json:"session"`
	HostOnly       bool    `json:"hostOnly"`
}
