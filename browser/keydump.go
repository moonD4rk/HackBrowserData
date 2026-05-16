package browser

import (
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
