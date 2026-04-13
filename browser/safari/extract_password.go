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
			CreatedAt: p.Created,
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

// getInternetPasswords reads InternetPassword records directly from the
// macOS login keychain. See rfcs/006-key-retrieval-mechanisms.md §7 for why
// Safari owns this path instead of routing through crypto/keyretriever.
func getInternetPasswords(keychainPassword string) ([]keychainbreaker.InternetPassword, error) {
	kc, err := keychainbreaker.Open()
	if err != nil {
		return nil, fmt.Errorf("open keychain: %w", err)
	}

	if keychainPassword != "" {
		if err := kc.TryUnlock(keychainbreaker.WithPassword(keychainPassword)); err != nil {
			log.Warnf("Safari passwords will be exported as metadata only (without plaintext passwords)")
			log.Debugf("keychain unlock detail: %v", err)
		}
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

	// Convert macOS Keychain FourCC protocol code to URL scheme.
	// Only "htps" needs special mapping; others just need space trimming.
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
