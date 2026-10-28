package safari

import (
	"context"
	"fmt"

	"github.com/moond4rk/keychainbreaker"

	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// passwordKeychain is today's direct read of the login keychain, handed to the Safari base until the switch-over
// gives every reader the run's one handle.
type passwordKeychain string

func (pw passwordKeychain) InternetPasswords(context.Context) ([]keychainbreaker.InternetPassword, error) {
	return getInternetPasswords(string(pw))
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
