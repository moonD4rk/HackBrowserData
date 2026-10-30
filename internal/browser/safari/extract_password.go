package safari

import (
	"context"
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func extractPasswords(ctx context.Context, kc LoginKeychain, out browser.Collector) error {
	passwords, err := kc.InternetPasswords(ctx)
	if err != nil {
		return err
	}

	var logins []record.Login
	for _, p := range passwords {
		url := buildURL(p.Protocol, p.Server, p.Port, p.Path)
		if url == "" || p.Account == "" {
			continue
		}
		logins = append(logins, record.Login{
			URL:       url,
			Username:  p.Account,
			Password:  p.PlainPassword,
			CreatedAt: p.Created.UTC(),
			Store:     record.StoreLocal,
		})
	}
	emit(out, logins)
	return nil
}

// buildURL constructs a URL from InternetPassword fields.
func buildURL(protocol, server string, port uint32, path string) string {
	if server == "" {
		return ""
	}

	// macOS Keychain stores the protocol as a FourCC code; only "htps" needs remapping, others just trim padding.
	scheme := strings.TrimRight(protocol, " ")
	if scheme == "" || scheme == "htps" {
		scheme = "https"
	}

	url := scheme + "://" + server

	defaultPorts := map[string]uint32{"https": 443, "http": 80, "ftp": 21}
	if port > 0 && port != defaultPorts[scheme] {
		url += fmt.Sprintf(":%d", port)
	}

	if path != "" && path != "/" {
		url += path
	}
	return url
}
