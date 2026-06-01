package safari

import (
	"fmt"
	"sort"
	"strings"

	"github.com/moond4rk/keychainbreaker"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

func extractPasswords(keychainPassword string) ([]types.LoginEntry, error) {
	passwords, err := getInternetPasswords(keychainPassword)
	if err != nil {
		return nil, err
	}

	var logins []types.LoginEntry
	for _, p := range passwords {
		url := buildURL(p.Protocol, p.Server, p.Port, p.Path)
		if url == "" || p.Account == "" {
			continue
		}
		logins = append(logins, types.LoginEntry{
			URL:       url,
			Username:  p.Account,
			Password:  p.PlainPassword,
			CreatedAt: p.Created.UTC(),
		})
	}

	sort.Slice(logins, func(i, j int) bool {
		return logins[i].CreatedAt.After(logins[j].CreatedAt)
	})
	return logins, nil
}

func countPasswords(keychainPassword string) (int, error) {
	passwords, err := extractPasswords(keychainPassword)
	if err != nil {
		return 0, err
	}
	return len(passwords), nil
}

// getInternetPasswords reads InternetPassword records straight from the macOS login keychain (Safari owns its own key
// path, separate from the masterkey package). TryUnlock always runs — even without a password — so a locked keychain
// still yields metadata-only records (URL, account, blank password) instead of failing with ErrLocked.
func getInternetPasswords(keychainPassword string) ([]keychainbreaker.InternetPassword, error) {
	kc, err := keychainbreaker.Open()
	if err != nil {
		return nil, fmt.Errorf("open keychain: %w", err)
	}

	var unlockOpts []keychainbreaker.UnlockOption
	if keychainPassword != "" {
		unlockOpts = append(unlockOpts, keychainbreaker.WithPassword(keychainPassword))
	}
	if err := kc.TryUnlock(unlockOpts...); err != nil {
		log.Debugf("keychain unlock detail: %v", err)
	}

	passwords, err := kc.InternetPasswords()
	if err != nil {
		return nil, fmt.Errorf("extract internet passwords: %w", err)
	}
	return passwords, nil
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
