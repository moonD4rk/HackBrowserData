package safari

import (
	"fmt"
	"sort"

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

func getInternetPasswords(keychainPassword string) ([]keychainbreaker.InternetPassword, error) {
	kc, err := keychainbreaker.Open()
	if err != nil {
		return nil, fmt.Errorf("open keychain: %w", err)
	}

	if keychainPassword != "" {
		if err := kc.Unlock(keychainbreaker.WithPassword(keychainPassword)); err != nil {
			log.Warnf("unlock keychain: %v", err)
			return nil, fmt.Errorf("unlock keychain: %w", err)
		}
	} else {
		// Try unlock without password; may fail but TryUnlock won't block.
		if err := kc.TryUnlock(); err != nil {
			log.Debugf("keychain unlock without password: %v", err)
			return nil, fmt.Errorf("keychain password required for Safari passwords: %w", err)
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

	scheme := protocolToScheme(protocol)
	url := scheme + "://" + server
	if port > 0 && !isDefaultPort(scheme, port) {
		url += fmt.Sprintf(":%d", port)
	}
	if path != "" && path != "/" {
		url += path
	}
	return url
}

const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

// protocolToScheme converts a macOS Keychain protocol FourCC code to a URL scheme.
func protocolToScheme(protocol string) string {
	switch protocol {
	case "htps":
		return schemeHTTPS
	case "http":
		return schemeHTTP
	case "ftps":
		return "ftps"
	case "ftp ":
		return "ftp"
	case "smb ":
		return "smb"
	case "afp ":
		return "afp"
	default:
		if protocol != "" {
			return protocol
		}
		return schemeHTTPS
	}
}

func isDefaultPort(scheme string, port uint32) bool {
	switch scheme {
	case schemeHTTPS:
		return port == 443
	case schemeHTTP:
		return port == 80
	case "ftp":
		return port == 21
	default:
		return false
	}
}
