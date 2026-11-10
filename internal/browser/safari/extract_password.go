package safari

import (
	"context"
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// extractPasswords reads no file, so it always runs; without a usable password the keychain yields metadata only.
func extractPasswords(ctx context.Context, input browser.Input, out browser.Collector) error {
	loginKeychain := input.LoginKeychain()
	if loginKeychain == nil {
		return errNoKeychain
	}
	passwords, err := loginKeychain.InternetPasswords(ctx)
	if err != nil {
		return err
	}

	for _, internetPassword := range passwords {
		url := buildURL(internetPassword.Protocol, internetPassword.Server, internetPassword.Port, internetPassword.Path)
		if url == "" || internetPassword.Account == "" {
			continue
		}
		out.Add(&record.Login{
			URL:       url,
			Username:  internetPassword.Account,
			Password:  internetPassword.Password,
			CreatedAt: internetPassword.Created.UTC(),
			Store:     record.StoreLocal,
		})
	}
	return nil
}

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
