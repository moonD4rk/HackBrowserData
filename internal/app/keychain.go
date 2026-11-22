package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// loginKeychain is the run's one login keychain, shared by Chromium's v10 key and Safari's passwords; nil off macOS.
type loginKeychain interface {
	masterkey.LoginKeychain
	safari.LoginKeychain
	Err() error
}

// warnIfLocked speaks once the run is over, because every reader of a locked keychain quietly gets metadata only.
func warnIfLocked(ctx context.Context, loginKeychain loginKeychain, consequence string) {
	if loginKeychain == nil || ctx.Err() != nil {
		return
	}
	if err := loginKeychain.Err(); err != nil {
		log.Warnf("%v; %s", err, consequence)
	}
}
