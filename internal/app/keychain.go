package app

import (
	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// loginKeychain is the run's one login keychain, shared by Chromium's v10 key and Safari's passwords; nil off macOS.
type loginKeychain interface {
	masterkey.LoginKeychain
	safari.LoginKeychain
	Err() error
}

// keychainErr is read once the run is over, because every reader of a locked keychain quietly gets metadata only.
func keychainErr(loginKeychain loginKeychain) error {
	if loginKeychain == nil {
		return nil
	}
	return loginKeychain.Err()
}
