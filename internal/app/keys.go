package app

import (
	"context"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// DumpKeys keeps the keys that worked when a dir's others fail, and leaves out a dir with none.
// It fails only when the run is stopped, so a stopped run writes no keys.json.
func DumpKeys(ctx context.Context, discovery Discovery, options Options) (masterkey.Dump, error) {
	return dumpKeysWith(ctx, discovery, machineKeys(loginKeychain(options)))
}

func dumpKeysWith(ctx context.Context, discovery Discovery, keys masterKeyRetriever) (masterkey.Dump, error) {
	dump := masterkey.NewDump()
	for _, dir := range discovery.Dirs {
		if dir.Browser.Base != browser.Chromium {
			continue
		}
		set, err := retrieveKeys(ctx, dir, keys)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return masterkey.Dump{}, fmt.Errorf("run stopped: %w", ctxErr)
		}
		if err != nil {
			status := "partial"
			if len(set) == 0 {
				status = "failed"
			}
			log.Warnf("dump-keys: %s %s: %v", dir.Browser.DisplayName, status, err)
		}
		if len(set) == 0 {
			continue
		}
		vault := masterkey.Vault{
			Browser:     dir.Browser.Name,
			Kind:        chromium.VaultKind(dir.Browser),
			UserDataDir: dir.Path,
			Profiles:    profileNames(dir),
			Keys:        set,
		}
		if string(dir.ID) != dir.Browser.Name {
			vault.ID = string(dir.ID)
		}
		dump.Vaults = append(dump.Vaults, vault)
	}
	return dump, nil
}

func retrieveKeys(ctx context.Context, dir browser.UserDataDir, keys masterKeyRetriever) (masterkey.Set, error) {
	snapshotDir, err := snapshot.Temp()
	if err != nil {
		return nil, err
	}
	defer closeSnapshot(snapshotDir)
	var files []snapshot.File
	for _, fileSpec := range chromium.KeyFileSpecs() {
		if file, ok, err := firstOnDisk(dir, browser.Profile{}, fileSpec); err == nil && ok {
			files = append(files, file)
		}
	}
	if err := snapshotDir.Capture(ctx, files); err != nil {
		return nil, err
	}
	return keys(ctx, dir, snapshotDir.Snapshot(string(dir.ID)))
}

func profileNames(dir browser.UserDataDir) []string {
	names := make([]string, 0, len(dir.Profiles))
	for _, profile := range dir.Profiles {
		names = append(names, profile.Name)
	}
	return names
}
