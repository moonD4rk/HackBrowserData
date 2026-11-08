package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/moond4rk/hackbrowserdata/internal/atomicfile"
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// DumpKeysFlags are dumpkeys' command-line values as cobra parsed them.
type DumpKeysFlags struct {
	Browser  string
	Keychain KeychainFlags
}

func ValidateDumpKeys(flags DumpKeysFlags) (Options, error) {
	browserName, err := parseBrowser(flags.Browser)
	if err != nil {
		return Options{}, err
	}
	password, err := keychainPasswordFunc(flags.Keychain, os.Stdin)
	if err != nil {
		return Options{}, err
	}
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{
		Target:   Target{BrowserName: browserName, HomeDir: home},
		Keychain: KeychainConfig{HomeDir: home, Password: password},
	}, nil
}

// DumpKeys writes the key file to outputPath, or to stdout when it is "", and returns how many vaults it holds.
// A stopped run writes nothing, so it never leaves a keys.json behind.
func DumpKeys(ctx context.Context, discovery Discovery, keychainConfig KeychainConfig, stdout io.Writer, outputPath string) (int, error) {
	keyFile, err := dumpKeysWith(ctx, discovery, keysFromMachine(newLoginKeychain(keychainConfig)))
	if err != nil {
		return 0, err
	}
	if outputPath == "" {
		return len(keyFile.Vaults), keyFile.Write(stdout)
	}
	if err := writeKeyFile(outputPath, keyFile); err != nil {
		return 0, err
	}
	return len(keyFile.Vaults), nil
}

// writeKeyFile replaces outputPath only once the file is complete, so a failure keeps the old keys.json.
func writeKeyFile(outputPath string, keyFile masterkey.KeyFile) error {
	if err := atomicfile.Write(outputPath, keyFile.Write); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	return nil
}

// dumpKeysWith keeps the keys that worked when a dir's others fail, and leaves out a dir with none.
// It fails only when the run is stopped.
func dumpKeysWith(ctx context.Context, discovery Discovery, keys dirKeysFunc) (masterkey.KeyFile, error) {
	keyFile := masterkey.NewKeyFile()
	for _, dir := range discovery.Dirs {
		if dir.Browser.Base != browser.Chromium {
			continue
		}
		set, err := retrieveKeys(ctx, dir, keys)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return masterkey.KeyFile{}, fmt.Errorf("run stopped: %w", ctxErr)
		}
		if err != nil {
			status := "partial"
			if len(set) == 0 {
				status = "failed"
			}
			log.Warnf("dumpkeys: %s %s: %v", dir.Browser.DisplayName, status, err)
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
		keyFile.Vaults = append(keyFile.Vaults, vault)
	}
	return keyFile, nil
}

func retrieveKeys(ctx context.Context, dir browser.UserDataDir, keys dirKeysFunc) (masterkey.Set, error) {
	snapshots, err := snapshot.Temp()
	if err != nil {
		return nil, err
	}
	defer closeSnapshot(snapshots)
	var files []snapshot.File
	for _, source := range chromium.KeySources() {
		if file, ok, err := firstOnDisk(dir, browser.Profile{}, source); err == nil && ok {
			files = append(files, file)
		}
	}
	if err := snapshots.Capture(ctx, files); err != nil {
		return nil, err
	}
	return keys(ctx, dir, snapshots.Snapshot(string(dir.ID)))
}

func profileNames(dir browser.UserDataDir) []string {
	names := make([]string, 0, len(dir.Profiles))
	for _, profile := range dir.Profiles {
		names = append(names, profile.Name)
	}
	return names
}
