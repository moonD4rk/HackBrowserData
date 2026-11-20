package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/binarycookies/v2"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func extractCookies(path string, out browser.Collector) error {
	jar, err := decodeBinaryCookies(path)
	if err != nil {
		return err
	}

	for _, cookie := range jar.Cookies() {
		hasExpire := !cookie.Expires.IsZero()
		out.Add(&record.Cookie{
			Host:         cookie.Domain,
			Path:         cookie.Path,
			Name:         cookie.Name,
			Value:        cookie.Value,
			IsSecure:     cookie.Secure(),
			IsHTTPOnly:   cookie.HTTPOnly(),
			HasExpire:    hasExpire,
			IsPersistent: hasExpire,
			ExpireAt:     cookie.Expires,
			CreatedAt:    cookie.Created,
		})
	}
	return nil
}

func decodeBinaryCookies(path string) (*binarycookies.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cookies file: %w", err)
	}
	defer file.Close()

	jar, err := binarycookies.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode cookies: %w", err)
	}
	return jar, nil
}
