package browser

import (
	"runtime"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
)

// BuildDump exports per-installation master keys; profiles sharing (Browser, UserDataDir) collapse into one Vault.
// Browsers without KeyManager (Firefox/Safari) are skipped. ExportKeys is invoked exactly once per installation
// regardless of profile count or success. Partial results (e.g. V10 retrieved, V20 failed) keep the usable tiers
// rather than discarding the vault, matching getMasterKeys' behavior on the extraction path — a Chrome 127+
// profile mixes v10 + v20 ciphertexts and a v20-only failure must not erase a usable v10 key.
func BuildDump(browsers []Browser) keyretriever.Dump {
	dump := keyretriever.NewDump()
	groups, order := groupByInstallation(browsers)
	for _, key := range order {
		g := groups[key]
		keys, err := g.km.ExportKeys()
		if err != nil {
			status := "partial"
			if !keys.HasAny() {
				status = "failed"
			}
			log.Warnf("dump-keys: %s/%s %s: %v", g.browser, g.profiles[0], status, err)
		}
		if !keys.HasAny() {
			continue
		}
		dump.Vaults = append(dump.Vaults, keyretriever.Vault{
			Browser:     g.browser,
			UserDataDir: g.userDataDir,
			Profiles:    g.profiles,
			Keys:        keys,
		})
	}
	return dump
}

type installGroup struct {
	browser, userDataDir string
	km                   KeyManager
	profiles             []string
}

// groupByInstallation collects browsers into per-installation groups keyed by (BrowserName, UserDataDir),
// preserving the discovery order of the first profile in each group. Non-KeyManager browsers are skipped.
// Doing the grouping up front (rather than checking dump.Vaults profile-by-profile) makes the resulting
// Profiles list complete and order-independent even if the group's ExportKeys later fails.
func groupByInstallation(browsers []Browser) (map[string]*installGroup, []string) {
	groups := make(map[string]*installGroup)
	var order []string
	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			continue
		}
		key := b.BrowserName() + "|" + b.UserDataDir()
		if g, exists := groups[key]; exists {
			g.profiles = append(g.profiles, b.ProfileName())
			continue
		}
		groups[key] = &installGroup{
			browser:     b.BrowserName(),
			userDataDir: b.UserDataDir(),
			km:          km,
			profiles:    []string{b.ProfileName()},
		}
		order = append(order, key)
	}
	return groups, order
}

// ApplyDump installs master keys from dump onto matching browsers, replacing each browser's default
// platform-native retrievers with StaticProviders backed by the Dump's bytes. Matching is by
// (BrowserName, UserDataDir) — the same key BuildDump groups by. When exact match fails (commonly a
// cross-host path mismatch: Windows backslash vs POSIX, or a relocated User Data dir via -p), falls
// back to the sole vault for that browser name when one exists. Browsers without a matching vault
// are warned and left untouched; non-KeyManager browsers (Firefox/Safari) are skipped silently.
func ApplyDump(browsers []Browser, dump keyretriever.Dump) {
	if dump.Host.OS != "" && dump.Host.OS != runtime.GOOS {
		log.Infof("apply-keys: dump created on %s/%s; current host is %s/%s",
			dump.Host.OS, dump.Host.Arch, runtime.GOOS, runtime.GOARCH)
	}
	vaultIndex := make(map[string]*keyretriever.Vault, len(dump.Vaults))
	vaultsByBrowser := make(map[string][]*keyretriever.Vault)
	for i := range dump.Vaults {
		v := &dump.Vaults[i]
		vaultIndex[v.Browser+"|"+v.UserDataDir] = v
		vaultsByBrowser[v.Browser] = append(vaultsByBrowser[v.Browser], v)
	}
	for _, b := range browsers {
		km, ok := b.(KeyManager)
		if !ok {
			continue
		}
		v, found := vaultIndex[b.BrowserName()+"|"+b.UserDataDir()]
		if !found {
			if candidates := vaultsByBrowser[b.BrowserName()]; len(candidates) == 1 {
				v = candidates[0]
				log.Infof("apply-keys: %s/%s using sole vault for browser (dump path %q != local %q)",
					b.BrowserName(), b.ProfileName(), v.UserDataDir, b.UserDataDir())
				found = true
			}
		}
		if !found {
			log.Warnf("apply-keys: %s/%s no matching vault in dump", b.BrowserName(), b.ProfileName())
			continue
		}
		km.SetKeyRetrievers(keyretriever.Retrievers{
			V10: maybeStaticProvider(v.Keys.V10),
			V11: maybeStaticProvider(v.Keys.V11),
			V20: maybeStaticProvider(v.Keys.V20),
		})
	}
}

// maybeStaticProvider wraps non-empty key bytes as a StaticProvider; an empty/nil key returns nil
// to preserve the "tier not applicable" signal NewMasterKeys expects.
func maybeStaticProvider(key []byte) keyretriever.KeyRetriever {
	if len(key) == 0 {
		return nil
	}
	return keyretriever.NewStaticProvider(key)
}
