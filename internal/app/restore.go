package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// RestoreFlags are restore's command-line values as cobra parsed them.
type RestoreFlags struct {
	KeyFile    string // a keys.json path, or "-" for stdin
	DataDir    string
	DataZip    string
	Browser    string
	Categories string
	Format     string
	OutputDir  string
	Zip        bool
}

type RestoreOptions struct {
	Vaults     []masterkey.Vault // the key file's vaults that -b selects
	Categories []category.Category
	Output     Output
}

// ValidateRestore checks restore's flags and reads its keys, from stdin for "-", before any data is opened. A restore's
// browsers are the key file's vaults, not this OS's table, so -b selects among them.
func ValidateRestore(flags RestoreFlags, stdin io.Reader) (RestoreOptions, error) {
	if flags.KeyFile == "" {
		return RestoreOptions{}, errors.New("requires --keys <file> (or - for stdin)")
	}
	if (flags.DataDir == "") == (flags.DataZip == "") {
		return RestoreOptions{}, errors.New("exactly one of --data-dir or --data-zip is required")
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return RestoreOptions{}, err
	}
	format, err := export.ParseFormat(flags.Format)
	if err != nil {
		return RestoreOptions{}, err
	}
	if err := checkOutputDir(flags.OutputDir); err != nil {
		return RestoreOptions{}, err
	}
	if err := checkRestoreData(flags.DataDir, flags.DataZip); err != nil {
		return RestoreOptions{}, err
	}
	keyFile, err := readKeyFile(flags.KeyFile, stdin)
	if err != nil {
		return RestoreOptions{}, err
	}
	vaults, err := selectVaults(keyFile.Vaults, flags.Browser)
	if err != nil {
		return RestoreOptions{}, err
	}
	return RestoreOptions{
		Vaults:     vaults,
		Categories: categories,
		Output:     Output{Dir: flags.OutputDir, Format: format, Zip: flags.Zip},
	}, nil
}

// selectVaults keeps the vaults of the browser -b names, or every vault when it names none.
func selectVaults(vaults []masterkey.Vault, browserName string) ([]masterkey.Vault, error) {
	name := strings.ToLower(strings.TrimSpace(browserName))
	if name == "" || name == allBrowsers {
		return vaults, nil
	}
	var selected []masterkey.Vault
	for _, vault := range vaults {
		if strings.EqualFold(vault.Browser, name) {
			selected = append(selected, vault)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no vault for browser %q in keys (have: %s)", name, vaultBrowserNames(vaults))
	}
	return selected, nil
}

func checkRestoreData(dataDir, dataZip string) error {
	if dataDir != "" {
		if !dirExists(dataDir) {
			return fmt.Errorf("data dir %q does not exist", dataDir)
		}
		return nil
	}
	if _, err := os.Stat(dataZip); err != nil {
		return fmt.Errorf("data zip: %w", err)
	}
	return nil
}

func readKeyFile(path string, stdin io.Reader) (masterkey.KeyFile, error) {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return masterkey.KeyFile{}, fmt.Errorf("open keys file %q: %w", path, err)
		}
		defer file.Close()
		reader = file
	}
	keyFile, err := masterkey.ReadKeyFile(reader)
	if err != nil {
		return masterkey.KeyFile{}, fmt.Errorf("read keys file %q: %w", path, err)
	}
	return keyFile, nil
}

// DiscoverRestore rebuilds the user data dirs under root, and their keys, from the vaults: each vault names a
// subdirectory of root, or root is one browser's User Data.
func DiscoverRestore(root string, vaults []masterkey.Vault) (Discovery, map[browser.UserDataDirID]masterkey.Set, error) {
	archived := false
	for _, vault := range vaults {
		if dirExists(filepath.Join(root, string(vaultUserDataDirID(vault)))) {
			archived = true
			break
		}
	}
	if !archived && len(vaults) > 1 {
		return Discovery{}, nil, fmt.Errorf("--data-dir %q has no per-browser subdir but keys has %d browsers; "+
			"point it at the archive root, or use -b <browser> for one browser's User Data (have: %s)",
			root, len(vaults), vaultBrowserNames(vaults))
	}
	var discovery Discovery
	keys := make(map[browser.UserDataDirID]masterkey.Set)
	for _, vault := range vaults {
		id := vaultUserDataDirID(vault)
		if _, duplicate := keys[id]; duplicate {
			log.Warnf("restore: keys hold %s twice, restoring the first", id)
			continue
		}
		path := root
		if archived {
			path = filepath.Join(root, string(id))
			if !dirExists(path) {
				log.Warnf("restore: %s has no data under %s, skipping", vault.Browser, path)
				continue
			}
		}
		dir, err := restoredDir(vault, id, path)
		if err != nil {
			log.Warnf("restore: %s: %v", vault.Browser, err)
			continue
		}
		discovery.Dirs = append(discovery.Dirs, dir)
		keys[id] = vault.Keys
	}
	return discovery, keys, nil
}

func restoredDir(vault masterkey.Vault, id browser.UserDataDirID, path string) (browser.UserDataDir, error) {
	restoredBrowser, err := vaultBrowser(vault)
	if err != nil {
		return browser.UserDataDir{}, err
	}
	dir, err := chromium.DiscoverAt(restoredBrowser, id, path)
	if err != nil {
		return browser.UserDataDir{}, fmt.Errorf("read %s: %w", path, err)
	}
	found := withData([]browser.UserDataDir{dir})
	if len(found) == 0 {
		return browser.UserDataDir{}, fmt.Errorf("found no profiles under %s", path)
	}
	return found[0], nil
}

// vaultBrowser searches the whole table, so a fork this OS lacks still restores. A browser this build does not know
// restores as it always did: the vault's kind picks the on-disk format, and its name stands in as the browser.
func vaultBrowser(vault masterkey.Vault) (browser.Browser, error) {
	name := strings.ToLower(vault.Browser)
	if namedBrowser, ok := browser.Lookup(name); ok && namedBrowser.Base == browser.Chromium {
		return namedBrowser, nil
	}
	formatName, err := chromium.BrowserNameForKind(vault.Kind, name)
	if err != nil {
		return browser.Browser{}, err
	}
	return browser.Browser{Name: formatName, DisplayName: vault.Browser, Base: browser.Chromium}, nil
}

// vaultUserDataDirID is the vault's directory in an archive: its ID for a second user data dir, else its browser name.
func vaultUserDataDirID(vault masterkey.Vault) browser.UserDataDirID {
	if vault.ID != "" {
		return browser.UserDataDirID(vault.ID)
	}
	return browser.UserDataDirID(strings.ToLower(vault.Browser))
}

func vaultBrowserNames(vaults []masterkey.Vault) string {
	keys := make([]string, 0, len(vaults))
	for _, vault := range vaults {
		keys = append(keys, strings.ToLower(vault.Browser))
	}
	return strings.Join(keys, ", ")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
