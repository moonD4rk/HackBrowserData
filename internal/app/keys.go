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

// DumpKeys retrieves each Chromium user data dir's keys for keys.json, capturing only its Local State. A dir whose
// keys partly fail keeps the rest, because a v20 failure must not cost a usable v10 key; a dir with none is left out.
// It fails only when the run is stopped, so a stopped run writes no keys.json.
func DumpKeys(ctx context.Context, d Discovery, req Request) (masterkey.Dump, error) {
	return dumpKeysWith(ctx, d, machineKeys(loginKeychain(req)))
}

func dumpKeysWith(ctx context.Context, d Discovery, keys keySource) (masterkey.Dump, error) {
	dump := masterkey.NewDump()
	for _, dir := range d.Dirs {
		if dir.Def.Base != browser.Chromium {
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
			log.Warnf("dump-keys: %s %s: %v", dir.Def.Name, status, err)
		}
		if len(set) == 0 {
			continue
		}
		v := masterkey.Vault{
			Browser:     dir.Def.ID,
			Kind:        chromium.VaultKind(dir.Def),
			UserDataDir: dir.Path,
			Profiles:    profileNames(dir),
			Keys:        set,
		}
		if string(dir.ID) != dir.Def.ID {
			v.ID = string(dir.ID)
		}
		dump.Vaults = append(dump.Vaults, v)
	}
	return dump, nil
}

func retrieveKeys(ctx context.Context, dir browser.UserDataDir, keys keySource) (masterkey.Set, error) {
	d, err := snapshot.Temp()
	if err != nil {
		return nil, err
	}
	defer closeSnapshot(d)
	var files []snapshot.File
	for _, src := range chromium.KeySources(dir) {
		if f, ok, err := firstOnDisk(dir, browser.Profile{}, src); err == nil && ok {
			files = append(files, f)
		}
	}
	if err := d.Capture(ctx, files); err != nil {
		return nil, err
	}
	return keys(ctx, dir, d.Snapshot(string(dir.ID)))
}

func profileNames(dir browser.UserDataDir) []string {
	names := make([]string, 0, len(dir.Profiles))
	for _, p := range dir.Profiles {
		names = append(names, p.Name)
	}
	return names
}
