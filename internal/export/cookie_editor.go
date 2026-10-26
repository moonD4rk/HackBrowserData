package export

import (
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var sameSiteNone = "no_restriction"

func cookieEditorValue(r record.Record) any {
	c, ok := r.(*record.Cookie)
	if !ok {
		return r
	}
	var expDate float64
	if !c.ExpireAt.IsZero() {
		expDate = float64(c.ExpireAt.Unix())
	}
	sameSite := &c.SameSite
	switch c.SameSite {
	case "none":
		sameSite = &sameSiteNone
	case "", "unspecified":
		sameSite = nil
	}
	return cookieEditorEntry{
		Domain:         c.Host,
		ExpirationDate: expDate,
		HTTPOnly:       c.IsHTTPOnly,
		Name:           c.Name,
		Path:           c.Path,
		Secure:         c.IsSecure,
		Value:          c.Value,
		SameSite:       sameSite,
		Session:        expDate == 0.,
		HostOnly:       !strings.HasPrefix(c.Host, "."),
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
