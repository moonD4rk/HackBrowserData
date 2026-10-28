//go:build darwin

package app

import (
	"context"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
)

// loginKeychain builds the run's one login keychain without opening it, so a run that never decrypts never prompts.
func loginKeychain(req Request) runKeychain {
	password := keychain.PasswordFunc(req.KeychainPassword)
	if password == nil {
		password = func(context.Context) (string, error) { return "", nil }
	}
	return keychain.New(filepath.Join(req.Home, "Library", "Keychains", "login.keychain-db"), password)
}
