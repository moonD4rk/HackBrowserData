package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/binarycookies"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func extractCookies(path string) ([]record.Cookie, error) {
	pages, err := decodeBinaryCookies(path)
	if err != nil {
		return nil, err
	}

	var cookies []record.Cookie
	for _, page := range pages {
		for _, c := range page.Cookies {
			hasExpire := !c.Expires.IsZero()
			// binarycookies returns time.Time in Local; normalize to UTC
			// so exported JSON matches Chromium/Firefox cookie output.
			cookies = append(cookies, record.Cookie{
				Host:         string(c.Domain),
				Path:         string(c.Path),
				Name:         string(c.Name),
				Value:        string(c.Value),
				IsSecure:     c.Secure,
				IsHTTPOnly:   c.HTTPOnly,
				HasExpire:    hasExpire,
				IsPersistent: hasExpire,
				ExpireAt:     c.Expires.UTC(),
				CreatedAt:    c.Creation.UTC(),
			})
		}
	}
	return cookies, nil
}

func countCookies(path string) (int, error) {
	pages, err := decodeBinaryCookies(path)
	if err != nil {
		return 0, err
	}
	var total int
	for _, page := range pages {
		total += len(page.Cookies)
	}
	return total, nil
}

func decodeBinaryCookies(path string) ([]binarycookies.Page, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cookies file: %w", err)
	}
	defer f.Close()

	jar := binarycookies.New(f)
	pages, err := jar.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode cookies: %w", err)
	}
	return pages, nil
}
