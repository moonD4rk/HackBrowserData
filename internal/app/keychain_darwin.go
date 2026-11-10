//go:build darwin

package app

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// newLoginKeychain builds the run's login keychain without opening it, so a run that never decrypts never prompts.
func newLoginKeychain(config KeychainConfig) loginKeychain {
	if config.Password == nil {
		return keychain.New(config.Path, func(context.Context) (string, error) { return "", nil })
	}
	return &reportingKeychain{Keychain: keychain.New(config.Path, keychain.PasswordFunc(config.Password))}
}

func loginKeychainPath(home string) string {
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}

// reportingKeychain says once why the keychain stayed locked, since every consumer then reads metadata only, quietly.
type reportingKeychain struct {
	*keychain.Keychain
	once sync.Once
}

func (k *reportingKeychain) GenericPassword(ctx context.Context, account string) ([]byte, error) {
	secret, err := k.Keychain.GenericPassword(ctx, account)
	k.warnLockedOnce()
	return secret, err
}

func (k *reportingKeychain) InternetPasswords(ctx context.Context) ([]keychain.InternetPassword, error) {
	passwords, err := k.Keychain.InternetPasswords(ctx)
	k.warnLockedOnce()
	return passwords, err
}

func (k *reportingKeychain) warnLockedOnce() {
	if err := k.Err(); err != nil {
		k.once.Do(func() { log.Warnf("%v; keychain-protected data is exported as metadata only", err) })
	}
}
