//go:build darwin

package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moond4rk/keychainbreaker"
	"golang.org/x/term"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// loginKeychainPath is the per-user login keychain the run reads Chromium v10 secrets from.
func loginKeychainPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}

// resolveKeychainPassword resolves the macOS login password (CLI flag, else TTY prompt) and verifies it against
// keychainbreaker. On any failure it returns "" so callers fall back to no-password mode rather than a known-bad credential.
func resolveKeychainPassword(flagPassword string) string {
	password := flagPassword
	if password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			log.Warnf("macOS login password not provided and stdin is not a TTY; keychain-protected data will be exported as metadata only")
			return ""
		}
		fmt.Fprint(os.Stderr, "Enter macOS login password: ")
		pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			log.Warnf("failed to read macOS login password: %v; keychain-protected data will be exported as metadata only", err)
			return ""
		}
		password = string(pwd)
	}

	if password == "" {
		log.Warnf("no macOS login password entered; keychain-protected data will be exported as metadata only")
		return ""
	}

	kc, err := keychainbreaker.Open()
	if err != nil {
		log.Warnf("keychain open failed: %v; keychain-protected data will be exported as metadata only", err)
		return ""
	}
	if err := kc.TryUnlock(keychainbreaker.WithPassword(password)); err != nil {
		log.Warnf("keychain unlock failed with provided password; keychain-protected data will be exported as metadata only")
		log.Debugf("keychain unlock detail: %v", err)
		return ""
	}

	return password
}

// newCredentialInjector lazily resolves the macOS login password and hands every Chromium browser the run's
// login keychain; `-b firefox` never triggers a prompt because resolution is skipped for browsers that need neither.
func newCredentialInjector(opts DiscoverOptions) browserInjector {
	var (
		password string
		kc       *keychain.Keychain
		resolved bool
	)
	return func(b Browser) {
		km, needsKeychain := b.(KeyManager)
		kps, needsKeychainPassword := b.(KeychainPasswordReceiver)
		if !needsKeychain && !needsKeychainPassword {
			return
		}
		if !resolved {
			password = resolveKeychainPassword(opts.KeychainPassword)
			pw := password
			kc = keychain.New(loginKeychainPath(), func(context.Context) (string, error) { return pw, nil })
			resolved = true
		}
		if needsKeychain {
			km.SetLocator(masterkey.Locator{Keychain: kc})
		}
		if needsKeychainPassword {
			kps.SetKeychainPassword(password)
		}
	}
}
