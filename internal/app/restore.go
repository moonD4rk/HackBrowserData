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
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// RestoreFlags are restore's command-line values as cobra parsed them.
type RestoreFlags struct {
	Keys       string // a keys.json path, or "-" for stdin
	DataDir    string
	DataZip    string
	Browser    string
	Categories string
	Format     string
	Dir        string
	Zip        bool
}

// ValidateRestore checks restore's flags and reads its keys before any data is opened. A restore's browsers are the
// dump's vaults, not this OS's table, so -b is checked against them.
func ValidateRestore(flags RestoreFlags) (Options, masterkey.Dump, error) {
	if flags.Keys == "" {
		return Options{}, masterkey.Dump{}, errors.New("requires --keys <file> (or - for stdin)")
	}
	if (flags.DataDir == "") == (flags.DataZip == "") {
		return Options{}, masterkey.Dump{}, errors.New("exactly one of --data-dir or --data-zip is required")
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return Options{}, masterkey.Dump{}, err
	}
	format, err := export.ParseFormat(flags.Format)
	if err != nil {
		return Options{}, masterkey.Dump{}, err
	}
	if err := checkOutputDir(flags.Dir); err != nil {
		return Options{}, masterkey.Dump{}, err
	}
	if err := checkRestoreData(flags.DataDir, flags.DataZip); err != nil {
		return Options{}, masterkey.Dump{}, err
	}
	dump, err := readKeys(flags.Keys)
	if err != nil {
		return Options{}, masterkey.Dump{}, err
	}
	filter := strings.ToLower(strings.TrimSpace(flags.Browser))
	if filter == "" {
		filter = allBrowsers
	}
	if filter != allBrowsers && len(vaultsOf(dump, filter)) == 0 {
		return Options{}, masterkey.Dump{}, fmt.Errorf("no vault for browser %q in keys (have: %s)", filter, vaultKeys(dump.Vaults))
	}
	options := Options{Browser: filter, Categories: categories, Format: format, Dir: flags.Dir, Zip: flags.Zip}
	return options, dump, nil
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

func readKeys(path string) (masterkey.Dump, error) {
	var reader io.Reader = os.Stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return masterkey.Dump{}, fmt.Errorf("open keys file %q: %w", path, err)
		}
		defer file.Close()
		reader = file
	}
	dump, err := masterkey.ReadJSON(reader)
	if err != nil {
		return masterkey.Dump{}, fmt.Errorf("read keys file %q: %w", path, err)
	}
	return dump, nil
}

// RestoreDiscovery rebuilds the user data dirs in data, and their keys from dump: from the manifest when data has one,
// else by today's rules, where the vaults' keys name subdirectories or data is one browser's User Data.
func RestoreDiscovery(
	data *snapshot.Dir, dump masterkey.Dump, options Options,
) (Discovery, map[browser.UserDataDirID]masterkey.Set, error) {
	vaults := vaultsOf(dump, options.Browser)
	manifest, ok, err := data.ReadManifest()
	if err != nil {
		return Discovery{}, nil, err
	}
	if ok {
		discovery, keys := fromManifest(data.Path(), manifest, vaults, options.Browser)
		return discovery, keys, nil
	}
	return fromLayout(data.Path(), vaults)
}

func fromManifest(
	root string, manifest snapshot.Manifest, vaults []masterkey.Vault, filter string,
) (Discovery, map[browser.UserDataDirID]masterkey.Set) {
	var discovery Discovery
	keys := make(map[browser.UserDataDirID]masterkey.Set)
	for _, manifestEntry := range manifest.Dirs {
		if filter != allBrowsers && manifestEntry.Browser != filter {
			continue
		}
		vault, ok := vaultOf(vaults, manifestEntry)
		if !ok {
			log.Warnf("restore: %s has no keys, skipping", manifestEntry.ID)
			continue
		}
		dir, err := manifestDir(root, manifestEntry, vault)
		if err != nil {
			log.Warnf("restore: %s: %v", manifestEntry.ID, err)
			continue
		}
		discovery.Dirs = append(discovery.Dirs, dir)
		keys[dir.ID] = vault.Keys
	}
	return discovery, keys
}

// vaultOf finds an entry's keys: the vault carrying its ID, or for a browser's first dir the vault of that browser.
func vaultOf(vaults []masterkey.Vault, manifestEntry snapshot.ManifestDir) (masterkey.Vault, bool) {
	for _, vault := range vaults {
		if vaultUserDataDirID(vault) == browser.UserDataDirID(manifestEntry.ID) {
			return vault, true
		}
	}
	return masterkey.Vault{}, false
}

// manifestDir trusts no path in the manifest that would leave its own directory of the archive.
func manifestDir(root string, manifestEntry snapshot.ManifestDir, vault masterkey.Vault) (browser.UserDataDir, error) {
	if manifestEntry.Base != browser.Chromium.String() {
		return browser.UserDataDir{}, fmt.Errorf("base %q is not restorable", manifestEntry.Base)
	}
	if !isPathElement(manifestEntry.ID) {
		return browser.UserDataDir{}, fmt.Errorf("id %q is not a directory name", manifestEntry.ID)
	}
	restoredBrowser, err := vaultBrowser(vault)
	if err != nil {
		return browser.UserDataDir{}, err
	}
	dir := browser.UserDataDir{
		ID: browser.UserDataDirID(manifestEntry.ID), Browser: restoredBrowser, Path: filepath.Join(root, manifestEntry.ID),
	}
	for _, manifestProfile := range manifestEntry.Profiles {
		if manifestProfile.Dir != "" && !filepath.IsLocal(filepath.FromSlash(manifestProfile.Dir)) {
			return browser.UserDataDir{}, fmt.Errorf("profile dir %q is outside the archive", manifestProfile.Dir)
		}
		dir.Profiles = append(dir.Profiles, browser.Profile{Name: manifestProfile.Name, Dir: manifestProfile.Dir})
	}
	return dir, nil
}

func fromLayout(root string, vaults []masterkey.Vault) (Discovery, map[browser.UserDataDirID]masterkey.Set, error) {
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
			root, len(vaults), vaultKeys(vaults))
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
		dir, err := layoutDir(vault, id, path)
		if err != nil {
			log.Warnf("restore: %s: %v", vault.Browser, err)
			continue
		}
		discovery.Dirs = append(discovery.Dirs, dir)
		keys[id] = vault.Keys
	}
	return discovery, keys, nil
}

func layoutDir(vault masterkey.Vault, id browser.UserDataDirID, path string) (browser.UserDataDir, error) {
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
	formatName, err := chromium.FormatName(vault.Kind, name)
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

func vaultsOf(dump masterkey.Dump, filter string) []masterkey.Vault {
	var out []masterkey.Vault
	for _, vault := range dump.Vaults {
		if filter == allBrowsers || strings.EqualFold(vault.Browser, filter) {
			out = append(out, vault)
		}
	}
	return out
}

func vaultKeys(vaults []masterkey.Vault) string {
	keys := make([]string, 0, len(vaults))
	for _, vault := range vaults {
		keys = append(keys, strings.ToLower(vault.Browser))
	}
	return strings.Join(keys, ", ")
}

func isPathElement(name string) bool {
	return name != "" && name != "." && filepath.IsLocal(name) && !strings.ContainsAny(name, `/\`)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
