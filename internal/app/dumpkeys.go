package app

import (
	"context"
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/internal/atomicfile"
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/keychain"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// DumpKeysFlags are dumpkeys' command-line values as cobra parsed them.
type DumpKeysFlags struct {
	Browser    string
	OutputPath string
}

type DumpKeysOptions struct {
	Browsers   []browser.Browser
	HomeDir    string
	OutputPath string // "" writes the key file to stdout
	// KeychainPassword is set by the caller once ValidateDumpKeys passes. It runs when the macOS login keychain first
	// opens; "" leaves it locked. Other systems ignore it.
	KeychainPassword keychain.PasswordFunc
}

func ValidateDumpKeys(flags DumpKeysFlags) (DumpKeysOptions, error) {
	browsers, err := parseBrowsers(flags.Browser)
	if err != nil {
		return DumpKeysOptions{}, err
	}
	if flags.OutputPath != "" {
		if err := checkOutputFile(flags.OutputPath); err != nil {
			return DumpKeysOptions{}, err
		}
	}
	home, err := homeDir()
	if err != nil {
		return DumpKeysOptions{}, err
	}
	return DumpKeysOptions{Browsers: browsers, HomeDir: home, OutputPath: flags.OutputPath}, nil
}

// KeyFileSummary is what DumpKeys reports about the key file it wrote.
type KeyFileSummary struct {
	Vaults      int   // vaults in the key file
	KeychainErr error // why the macOS login keychain stayed locked; nil when it opened or was never read
}

// DumpKeys writes the key file to options.OutputPath, or to stdout when it is "". A stopped run writes nothing, so it
// never leaves a keys.json behind.
func DumpKeys(ctx context.Context, options DumpKeysOptions, stdout io.Writer) (KeyFileSummary, error) {
	discovery := discover(ctx, options.Browsers, browser.DiscoverOptions{HomeDir: options.HomeDir})
	loginKeychain := newLoginKeychain(loginKeychainPath(options.HomeDir), options.KeychainPassword)
	keyFile, err := buildKeyFile(ctx, discovery, machineKeys{loginKeychain: loginKeychain})
	if err != nil {
		return KeyFileSummary{}, err
	}
	if options.OutputPath == "" {
		err = keyFile.Write(stdout)
	} else {
		err = writeKeyFile(options.OutputPath, keyFile)
	}
	if err != nil {
		return KeyFileSummary{}, err
	}
	return KeyFileSummary{Vaults: len(keyFile.Vaults), KeychainErr: keychainErr(loginKeychain)}, nil
}

// writeKeyFile replaces outputPath only once the file is complete, so a failure keeps the old keys.json.
func writeKeyFile(outputPath string, keyFile masterkey.KeyFile) error {
	if err := atomicfile.Write(outputPath, keyFile.Write); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	return nil
}

// buildKeyFile keeps the keys that worked when a dir's others fail, and leaves out a dir with none.
// It fails only when the run is stopped.
func buildKeyFile(ctx context.Context, discovery Discovery, keySource keySource) (masterkey.KeyFile, error) {
	keyFile := masterkey.NewKeyFile()
	for _, dir := range discovery.Dirs {
		if dir.Browser.Base != browser.Chromium {
			continue
		}
		retrieval := retrieveDirKeys(ctx, dir, keySource)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return masterkey.KeyFile{}, fmt.Errorf("run stopped: %w", ctxErr)
		}
		if retrieval.Err != nil {
			status := "partial"
			if len(retrieval.Keys) == 0 {
				status = "failed"
			}
			log.Warnf("dumpkeys: %s %s: %v", dir.Browser.DisplayName, status, retrieval.Err)
		}
		if len(retrieval.Keys) == 0 {
			continue
		}
		vault := masterkey.Vault{
			Browser:     dir.Browser.Name,
			Kind:        chromium.VaultKind(dir.Browser),
			UserDataDir: dir.Path,
			Profiles:    profileNames(dir),
			Keys:        retrieval.Keys,
		}
		if string(dir.ID) != dir.Browser.Name {
			vault.ID = string(dir.ID)
		}
		keyFile.Vaults = append(keyFile.Vaults, vault)
	}
	return keyFile, nil
}

func retrieveDirKeys(ctx context.Context, dir browser.UserDataDir, keySource keySource) masterkey.Retrieval {
	root, err := snapshot.NewRoot()
	if err != nil {
		return masterkey.Retrieval{Err: err}
	}
	defer closeRoot(root)
	var files []snapshot.File
	for _, source := range chromium.KeySources() {
		if file, ok, err := firstOnDisk(dir, source.RelativePaths(), source.IsDir); err == nil && ok {
			files = append(files, file)
		}
	}
	if err := root.Capture(ctx, files); err != nil {
		return masterkey.Retrieval{Err: err}
	}
	return keySource.MasterKeys(ctx, dir, root.Snapshot(string(dir.ID)))
}

func profileNames(dir browser.UserDataDir) []string {
	names := make([]string, 0, len(dir.Profiles))
	for _, profile := range dir.Profiles {
		names = append(names, profile.Name)
	}
	return names
}
