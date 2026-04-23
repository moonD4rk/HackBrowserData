package safari

import (
	"fmt"
	"os"
	"sort"

	"github.com/moond4rk/binarycookies"

	"github.com/moond4rk/hackbrowserdata/types"
)

func extractCookies(path string) ([]types.CookieEntry, error) {
	pages, err := decodeBinaryCookies(path)
	if err != nil {
		return nil, err
	}

	var cookies []types.CookieEntry
	for _, page := range pages {
		for _, c := range page.Cookies {
			hasExpire := !c.Expires.IsZero()
			// binarycookies returns time.Time in Local; normalize to UTC
			// so exported JSON matches Chromium/Firefox cookie output.
			cookies = append(cookies, types.CookieEntry{
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

	sort.Slice(cookies, func(i, j int) bool {
		return cookies[i].CreatedAt.After(cookies[j].CreatedAt)
	})
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
