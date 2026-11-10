package app

import (
	"context"
	"sync"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// loginKeychain is the run's one login keychain, shared by Chromium's v10 key and Safari's passwords; nil off macOS.
type loginKeychain interface {
	masterkey.LoginKeychain
	browser.LoginKeychain
}

type dirKeysFunc func(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) (masterkey.Set, error)

// dirKeys retrieves a dir's keys on first use, for all its profiles; Go 1.20 has no sync.OnceValues.
type dirKeys struct {
	once        sync.Once
	retrieve    dirKeysFunc // nil for a dir whose base keeps no master keys
	dir         browser.UserDataDir
	dirSnapshot snapshot.Snapshot
	keys        masterkey.Set
	err         error
}

func (k *dirKeys) get(ctx context.Context) masterkey.Set {
	k.once.Do(func() {
		if k.retrieve != nil {
			k.keys, k.err = k.retrieve(ctx, k.dir, k.dirSnapshot)
		}
	})
	return k.keys
}

// keysFromMachine asks this machine's secret stores; on macOS that includes the run's login keychain.
func keysFromMachine(keychain masterkey.LoginKeychain) dirKeysFunc {
	return func(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) (masterkey.Set, error) {
		locator, err := chromium.KeyLocator(dir, dirSnapshot)
		if err != nil {
			return nil, err
		}
		locator.Keychain = keychain
		return masterkey.RetrieveAll(ctx, locator)
	}
}

func keysFromFile(keys map[browser.UserDataDirID]masterkey.Set) dirKeysFunc {
	return func(_ context.Context, dir browser.UserDataDir, _ snapshot.Snapshot) (masterkey.Set, error) {
		return keys[dir.ID], nil
	}
}
