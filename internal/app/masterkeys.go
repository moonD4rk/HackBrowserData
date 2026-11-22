package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// keySource is where a run gets each Chromium dir's master keys: this machine, a key file, or nowhere.
type keySource interface {
	MasterKeys(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) masterkey.Retrieval
}

// machineKeys asks this machine's secret stores; on macOS that includes the run's login keychain.
type machineKeys struct {
	loginKeychain masterkey.LoginKeychain
}

func (k machineKeys) MasterKeys(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) masterkey.Retrieval {
	locator, err := chromium.KeyLocator(dir, dirSnapshot)
	if err != nil {
		return masterkey.Retrieval{Err: err}
	}
	locator.Keychain = k.loginKeychain
	return masterkey.RetrieveAll(ctx, locator)
}

// keyFileKeys are a key file's master keys by dir; a dir it lacks gets none, so its encrypted values stay empty.
type keyFileKeys map[browser.UserDataDirID]masterkey.Set

func (k keyFileKeys) MasterKeys(_ context.Context, dir browser.UserDataDir, _ snapshot.Snapshot) masterkey.Retrieval {
	return masterkey.Retrieval{Keys: k[dir.ID]}
}

// noKeys lets list --detail count encrypted rows without retrieving a key, so counting never prompts.
type noKeys struct{}

func (noKeys) MasterKeys(context.Context, browser.UserDataDir, snapshot.Snapshot) masterkey.Retrieval {
	return masterkey.Retrieval{}
}
