//go:build darwin

package app

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
)

// newLoginKeychain builds the run's login keychain without opening it, so a run that never decrypts never prompts.
func newLoginKeychain(path string, password keychain.PasswordFunc) loginKeychain {
	return keychain.New(path, password)
}

func loginKeychainPath(home string) string {
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}
