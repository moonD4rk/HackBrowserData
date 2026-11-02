// Package keychain owns the user's macOS login keychain: one handle per run, opened on first use.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/moond4rk/keychainbreaker"
)

// PasswordFunc yields the login password; "" means none was supplied.
type PasswordFunc func(ctx context.Context) (string, error)

// ErrLocked reports that no usable password was supplied, so the keychain yields metadata only.
var ErrLocked = errors.New("login keychain is locked")

var errNotFound = errors.New("not found in login keychain")

type Keychain struct {
	path     string
	password PasswordFunc

	openOnce sync.Once
	opened   *keychainbreaker.Keychain
	err      error

	genericOnce sync.Once
	generic     []keychainbreaker.GenericPassword
	genericErr  error
}

// New does no I/O, so building a handle never prompts; the first query opens the keychain.
func New(path string, password PasswordFunc) *Keychain {
	return &Keychain{path: path, password: password}
}

func (k *Keychain) GenericPassword(ctx context.Context, account string) ([]byte, error) {
	if err := k.open(ctx); err != nil {
		return nil, err
	}
	if !k.opened.Unlocked() {
		if k.err != nil {
			return nil, k.err
		}
		return nil, ErrLocked
	}
	k.genericOnce.Do(func() {
		k.generic, k.genericErr = k.opened.GenericPasswords()
	})
	if k.genericErr != nil {
		return nil, fmt.Errorf("read generic passwords: %w", k.genericErr)
	}
	for _, genericPassword := range k.generic {
		if genericPassword.Account == account {
			return genericPassword.Password, nil
		}
	}
	return nil, fmt.Errorf("%q: %w", account, errNotFound)
}

// InternetPasswords leaves the secrets empty when the keychain is locked.
func (k *Keychain) InternetPasswords(ctx context.Context) ([]keychainbreaker.InternetPassword, error) {
	if err := k.open(ctx); err != nil {
		return nil, err
	}
	passwords, err := k.opened.InternetPasswords()
	if err != nil {
		return nil, fmt.Errorf("read internet passwords: %w", err)
	}
	return passwords, nil
}

// Err is why the keychain is locked or unreadable; nil when it is open or was never used.
func (k *Keychain) Err() error {
	return k.err
}

func (k *Keychain) open(ctx context.Context) error {
	k.openOnce.Do(func() {
		opened, err := keychainbreaker.Open(keychainbreaker.WithFile(k.path))
		if err != nil {
			k.err = fmt.Errorf("open login keychain: %w", err)
			return
		}
		k.opened = opened
		k.err = k.unlock(ctx)
	})
	if k.opened == nil {
		return k.err
	}
	return nil
}

// unlock uses TryUnlock even without a password, so a locked keychain still yields its metadata.
func (k *Keychain) unlock(ctx context.Context) error {
	password, err := k.password(ctx)
	if err != nil {
		if tryErr := k.opened.TryUnlock(); tryErr != nil {
			return fmt.Errorf("%w: read password: %w", ErrLocked, errors.Join(err, tryErr))
		}
		return fmt.Errorf("%w: read password: %w", ErrLocked, err)
	}
	if password == "" {
		if err := k.opened.TryUnlock(); err != nil {
			return fmt.Errorf("%w: %w", ErrLocked, err)
		}
		return ErrLocked
	}
	if err := k.opened.TryUnlock(keychainbreaker.WithPassword(password)); err != nil {
		return fmt.Errorf("%w: %w", ErrLocked, err)
	}
	return nil
}
