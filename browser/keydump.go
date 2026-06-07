package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

// BuildDump exports one Vault per installation (Firefox/Safari, lacking KeyManager, are skipped).
// Partial results are kept — a Chrome 127+ profile mixes v10+v20, so a v20-only failure must not
// discard a usable v10 key.
func BuildDump(browsers []Browser) masterkey.Dump {
	dump := masterkey.NewDump()
	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			continue
		}
		mk, err := km.ExportKeys()
		if err != nil {
			status := "partial"
			if !mk.HasAny() {
				status = "failed"
			}
			log.Warnf("dump-keys: %s %s: %v", b.BrowserName(), status, err)
		}
		if !mk.HasAny() {
			continue
		}
		kind, err := kindToDump(km.Kind())
		if err != nil {
			log.Warnf("dump-keys: %s: %v", b.BrowserName(), err)
			continue
		}
		dump.Vaults = append(dump.Vaults, masterkey.Vault{
			Browser:     km.BrowserKey(),
			Kind:        kind,
			UserDataDir: b.UserDataDir(),
			Profiles:    profileNames(b),
			Keys:        mk,
		})
	}
	return dump
}

func profileNames(b Browser) []string {
	profiles := b.Profiles()
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return names
}

// BuildFromDump reconstructs Chromium engines straight from a dump's vaults, rooted at copied data
// instead of the local platform table — this is what lets an analyst host decrypt a browser its OS
// never installs. filter is a browser key ("" or "all" = every vault); a filter matching no vault is
// an error rather than silent empty output.
//
// Data layout is resolved two ways. When dataDir holds per-key subdirs (the archive layout), each
// vault is rooted at dataDir/<key>. Otherwise dataDir is treated as one browser's User Data (a
// hand-copied folder), which is unambiguous only for a single vault — so filter must pick one.
func BuildFromDump(dump masterkey.Dump, dataDir, filter string) ([]Browser, error) {
	filter = strings.ToLower(filter)
	if filter == "all" {
		filter = ""
	}

	var selected []masterkey.Vault
	for _, v := range dump.Vaults {
		if filter != "" && !strings.EqualFold(v.Browser, filter) {
			continue
		}
		selected = append(selected, v)
	}
	if filter != "" && len(selected) == 0 {
		return nil, fmt.Errorf("no vault for browser %q in keys (have: %s)", filter, vaultKeys(dump))
	}

	if !dirExists(dataDir) {
		return nil, fmt.Errorf("data dir %q does not exist", dataDir)
	}

	archiveLayout := isArchiveLayout(dataDir, selected)
	if !archiveLayout && len(selected) > 1 {
		return nil, fmt.Errorf("--data-dir %q has no per-browser subdir but keys has %d browsers; "+
			"point it at the archive root, or use -b <browser> for one browser's User Data (have: %s)",
			dataDir, len(selected), vaultKeys(dump))
	}

	var browsers []Browser
	for _, v := range selected {
		root := dataDir
		if archiveLayout {
			root = filepath.Join(dataDir, strings.ToLower(v.Browser))
			if !dirExists(root) {
				log.Warnf("restore: %s has no data under %s, skipping", v.Browser, root)
				continue
			}
		}
		kind, err := kindFromDump(v.Kind)
		if err != nil {
			log.Warnf("restore: %s: %v", v.Browser, err)
			continue
		}
		cfg := types.BrowserConfig{
			Key:         strings.ToLower(v.Browser),
			Name:        v.Browser,
			Kind:        kind,
			UserDataDir: root,
		}
		b, err := newBrowser(cfg)
		if err != nil {
			log.Errorf("restore: build %s: %v", v.Browser, err)
			continue
		}
		if b == nil {
			log.Warnf("restore: %s found no profiles under %s", v.Browser, root)
			continue
		}
		if km, ok := b.(KeyManager); ok {
			km.SetRetrievers(retrieversFromKeys(v.Keys))
		}
		browsers = append(browsers, b)
	}
	return browsers, nil
}

// isArchiveLayout reports whether dataDir uses the archive layout — one per-browser subdir named by
// the vault key — rather than a raw single-browser User Data copy.
func isArchiveLayout(dataDir string, vaults []masterkey.Vault) bool {
	for _, v := range vaults {
		if dirExists(filepath.Join(dataDir, strings.ToLower(v.Browser))) {
			return true
		}
	}
	return false
}

func vaultKeys(dump masterkey.Dump) string {
	keys := make([]string, 0, len(dump.Vaults))
	for _, v := range dump.Vaults {
		keys = append(keys, strings.ToLower(v.Browser))
	}
	return strings.Join(keys, ", ")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// maybeStaticRetriever wraps non-empty key bytes as a StaticRetriever; an empty/nil key returns nil
// to preserve the "tier not applicable" signal NewMasterKeys expects.
func maybeStaticRetriever(key []byte) masterkey.Retriever {
	if len(key) == 0 {
		return nil
	}
	return masterkey.NewStaticRetriever(key)
}

// retrieversFromKeys maps a vault's per-tier key bytes to static retrievers; an absent tier stays nil
// so NewMasterKeys keeps treating it as "not applicable".
func retrieversFromKeys(mk masterkey.MasterKeys) masterkey.Retrievers {
	return masterkey.Retrievers{
		V10: maybeStaticRetriever(mk.V10),
		V11: maybeStaticRetriever(mk.V11),
		V20: maybeStaticRetriever(mk.V20),
	}
}

func kindToDump(k types.BrowserKind) (string, error) {
	switch k {
	case types.Chromium, types.ChromiumYandex, types.ChromiumOpera:
		return k.String(), nil
	default:
		return "", fmt.Errorf("engine kind %s is not exportable", k)
	}
}

// dumpableKinds are the engine kinds a vault may carry; kindFromDump reverses BrowserKind.String()
// over exactly these, keeping the wire vocabulary single-sourced in the types enum.
var dumpableKinds = []types.BrowserKind{types.Chromium, types.ChromiumYandex, types.ChromiumOpera}

func kindFromDump(s string) (types.BrowserKind, error) {
	for _, k := range dumpableKinds {
		if k.String() == s {
			return k, nil
		}
	}
	return 0, fmt.Errorf("unknown engine kind %q", s)
}
