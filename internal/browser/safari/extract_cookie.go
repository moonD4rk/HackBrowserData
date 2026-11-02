package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/binarycookies"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func extractCookies(path string, out browser.Collector) error {
	pages, err := decodeBinaryCookies(path)
	if err != nil {
		return err
	}

	var cookies []record.Cookie
	for _, page := range pages {
		for _, cookie := range page.Cookies {
			hasExpire := !cookie.Expires.IsZero()
			// binarycookies returns local times; UTC keeps the JSON in line with Chromium's and Firefox's cookies.
			cookies = append(cookies, record.Cookie{
				Host:         string(cookie.Domain),
				Path:         string(cookie.Path),
				Name:         string(cookie.Name),
				Value:        string(cookie.Value),
				IsSecure:     cookie.Secure,
				IsHTTPOnly:   cookie.HTTPOnly,
				HasExpire:    hasExpire,
				IsPersistent: hasExpire,
				ExpireAt:     cookie.Expires.UTC(),
				CreatedAt:    cookie.Creation.UTC(),
			})
		}
	}
	browser.AddAll(out, cookies)
	return nil
}

func decodeBinaryCookies(path string) ([]binarycookies.Page, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cookies file: %w", err)
	}
	defer file.Close()

	jar := binarycookies.New(file)
	pages, err := jar.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode cookies: %w", err)
	}
	return pages, nil
}
