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
func ValidateRestore(f RestoreFlags) (Request, masterkey.Dump, error) {
	if f.Keys == "" {
		return Request{}, masterkey.Dump{}, errors.New("requires --keys <file> (or - for stdin)")
	}
	if (f.DataDir == "") == (f.DataZip == "") {
		return Request{}, masterkey.Dump{}, errors.New("exactly one of --data-dir or --data-zip is required")
	}
	categories, err := parseCategories(f.Categories)
	if err != nil {
		return Request{}, masterkey.Dump{}, err
	}
	format, err := export.ParseFormat(f.Format)
	if err != nil {
		return Request{}, masterkey.Dump{}, err
	}
	if err := checkOutputDir(f.Dir); err != nil {
		return Request{}, masterkey.Dump{}, err
	}
	if err := checkRestoreData(f.DataDir, f.DataZip); err != nil {
		return Request{}, masterkey.Dump{}, err
	}
	dump, err := readKeys(f.Keys)
	if err != nil {
		return Request{}, masterkey.Dump{}, err
	}
	filter := strings.ToLower(strings.TrimSpace(f.Browser))
	if filter == "" {
		filter = allBrowsers
	}
	if filter != allBrowsers && len(vaultsOf(dump, filter)) == 0 {
		return Request{}, masterkey.Dump{}, fmt.Errorf("no vault for browser %q in keys (have: %s)", filter, vaultKeys(dump.Vaults))
	}
	req := Request{Browser: filter, Categories: categories, Format: format, Dir: f.Dir, Zip: f.Zip}
	return req, dump, nil
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
	var r io.Reader = os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return masterkey.Dump{}, fmt.Errorf("open keys file %q: %w", path, err)
		}
		defer f.Close()
		r = f
	}
	dump, err := masterkey.ReadJSON(r)
	if err != nil {
		return masterkey.Dump{}, fmt.Errorf("read keys file %q: %w", path, err)
	}
	return dump, nil
}

// RestoreDiscovery rebuilds the user data dirs in data, and their keys from dump: from the manifest when data has one,
// else by today's rules, where the vaults' keys name subdirectories or data is one browser's User Data.
func RestoreDiscovery(data *snapshot.Dir, dump masterkey.Dump, req Request) (Discovery, map[browser.DirID]masterkey.Set, error) {
	vaults := vaultsOf(dump, req.Browser)
	m, ok, err := data.ReadManifest()
	if err != nil {
		return Discovery{}, nil, err
	}
	if ok {
		d, keys := fromManifest(data.Path(), m, vaults, req.Browser)
		return d, keys, nil
	}
	return fromLayout(data.Path(), vaults)
}

func fromManifest(root string, m snapshot.Manifest, vaults []masterkey.Vault, filter string) (Discovery, map[browser.DirID]masterkey.Set) {
	var d Discovery
	keys := make(map[browser.DirID]masterkey.Set)
	for _, md := range m.Dirs {
		if filter != allBrowsers && md.Browser != filter {
			continue
		}
		v, ok := vaultOf(vaults, md)
		if !ok {
			log.Warnf("restore: %s has no keys, skipping", md.ID)
			continue
		}
		dir, err := manifestDir(root, md, v)
		if err != nil {
			log.Warnf("restore: %s: %v", md.ID, err)
			continue
		}
		d.Dirs = append(d.Dirs, dir)
		keys[dir.ID] = v.Keys
	}
	return d, keys
}

// vaultOf finds md's keys: the vault carrying its ID, or for a browser's first dir the vault of that browser.
func vaultOf(vaults []masterkey.Vault, md snapshot.ManifestDir) (masterkey.Vault, bool) {
	for _, v := range vaults {
		if vaultDirID(v) == browser.DirID(md.ID) {
			return v, true
		}
	}
	return masterkey.Vault{}, false
}

// manifestDir trusts no path in the manifest that would leave its own directory of the archive.
func manifestDir(root string, md snapshot.ManifestDir, v masterkey.Vault) (browser.UserDataDir, error) {
	if md.Base != browser.Chromium.String() {
		return browser.UserDataDir{}, fmt.Errorf("base %q is not restorable", md.Base)
	}
	if !isPathElement(md.ID) {
		return browser.UserDataDir{}, fmt.Errorf("id %q is not a directory name", md.ID)
	}
	def, err := vaultDef(v)
	if err != nil {
		return browser.UserDataDir{}, err
	}
	dir := browser.UserDataDir{ID: browser.DirID(md.ID), Def: def, Path: filepath.Join(root, md.ID)}
	for _, p := range md.Profiles {
		if p.Dir != "" && !filepath.IsLocal(filepath.FromSlash(p.Dir)) {
			return browser.UserDataDir{}, fmt.Errorf("profile dir %q is outside the archive", p.Dir)
		}
		dir.Profiles = append(dir.Profiles, browser.Profile{Name: p.Name, Dir: p.Dir})
	}
	return dir, nil
}

func fromLayout(root string, vaults []masterkey.Vault) (Discovery, map[browser.DirID]masterkey.Set, error) {
	archived := false
	for _, v := range vaults {
		if dirExists(filepath.Join(root, string(vaultDirID(v)))) {
			archived = true
			break
		}
	}
	if !archived && len(vaults) > 1 {
		return Discovery{}, nil, fmt.Errorf("--data-dir %q has no per-browser subdir but keys has %d browsers; "+
			"point it at the archive root, or use -b <browser> for one browser's User Data (have: %s)",
			root, len(vaults), vaultKeys(vaults))
	}
	var d Discovery
	keys := make(map[browser.DirID]masterkey.Set)
	for _, v := range vaults {
		id := vaultDirID(v)
		if _, dup := keys[id]; dup {
			log.Warnf("restore: keys hold %s twice, restoring the first", id)
			continue
		}
		path := root
		if archived {
			path = filepath.Join(root, string(id))
			if !dirExists(path) {
				log.Warnf("restore: %s has no data under %s, skipping", v.Browser, path)
				continue
			}
		}
		dir, err := layoutDir(v, id, path)
		if err != nil {
			log.Warnf("restore: %s: %v", v.Browser, err)
			continue
		}
		d.Dirs = append(d.Dirs, dir)
		keys[id] = v.Keys
	}
	return d, keys, nil
}

func layoutDir(v masterkey.Vault, id browser.DirID, path string) (browser.UserDataDir, error) {
	def, err := vaultDef(v)
	if err != nil {
		return browser.UserDataDir{}, err
	}
	dir, err := chromium.DiscoverAt(def, id, path)
	if err != nil {
		return browser.UserDataDir{}, fmt.Errorf("read %s: %w", path, err)
	}
	found := withData([]browser.UserDataDir{dir})
	if len(found) == 0 {
		return browser.UserDataDir{}, fmt.Errorf("found no profiles under %s", path)
	}
	return found[0], nil
}

// vaultDef finds a vault's browser in the whole table, so a fork this OS lacks still restores. A key this build does
// not know restores as it always did: the vault's kind picks the on-disk format, and the key is stamped as the browser.
func vaultDef(v masterkey.Vault) (browser.Def, error) {
	key := strings.ToLower(v.Browser)
	if def, ok := browser.Lookup(key); ok && def.Base == browser.Chromium {
		return def, nil
	}
	id, err := chromium.FormatID(v.Kind, key)
	if err != nil {
		return browser.Def{}, err
	}
	return browser.Def{ID: id, Name: v.Browser, Base: browser.Chromium}, nil
}

// vaultDirID is the vault's directory in an archive: its ID for a second user data dir, else its browser key.
func vaultDirID(v masterkey.Vault) browser.DirID {
	if v.ID != "" {
		return browser.DirID(v.ID)
	}
	return browser.DirID(strings.ToLower(v.Browser))
}

func vaultsOf(dump masterkey.Dump, filter string) []masterkey.Vault {
	var out []masterkey.Vault
	for _, v := range dump.Vaults {
		if filter == allBrowsers || strings.EqualFold(v.Browser, filter) {
			out = append(out, v)
		}
	}
	return out
}

func vaultKeys(vaults []masterkey.Vault) string {
	keys := make([]string, 0, len(vaults))
	for _, v := range vaults {
		keys = append(keys, strings.ToLower(v.Browser))
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
