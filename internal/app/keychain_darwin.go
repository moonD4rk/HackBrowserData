//go:build darwin

package app

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/moond4rk/keychainbreaker"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// loginKeychain builds the run's one login keychain without opening it, so a run that never decrypts never prompts.
func loginKeychain(req Request) runKeychain {
	path := filepath.Join(req.Home, "Library", "Keychains", "login.keychain-db")
	if req.KeychainPassword == nil {
		return keychain.New(path, func(context.Context) (string, error) { return "", nil })
	}
	return &reportedKeychain{Keychain: keychain.New(path, keychain.PasswordFunc(req.KeychainPassword))}
}

// reportedKeychain says once why the keychain stayed locked, since every consumer then reads metadata only, quietly.
type reportedKeychain struct {
	*keychain.Keychain
	once sync.Once
}

func (k *reportedKeychain) GenericPassword(ctx context.Context, account string) ([]byte, error) {
	secret, err := k.Keychain.GenericPassword(ctx, account)
	k.report()
	return secret, err
}

func (k *reportedKeychain) InternetPasswords(ctx context.Context) ([]keychainbreaker.InternetPassword, error) {
	passwords, err := k.Keychain.InternetPasswords(ctx)
	k.report()
	return passwords, err
}

func (k *reportedKeychain) report() {
	if err := k.Err(); err != nil {
		k.once.Do(func() { log.Warnf("%v; keychain-protected data is exported as metadata only", err) })
	}
}
