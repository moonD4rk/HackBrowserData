package browser

import (
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
)

// BuildDump exports per-installation master keys; profiles sharing (Browser, UserDataDir) collapse into one Vault.
// Browsers without KeyManager (Firefox/Safari) are skipped; per-vault ExportKeys failures drop just that vault.
func BuildDump(browsers []Browser) keyretriever.Dump {
	dump := keyretriever.NewDump()
	for _, b := range browsers {
		if km, ok := b.(KeyManager); ok {
			addBrowserToDump(&dump, b, km)
		}
	}
	return dump
}

func addBrowserToDump(dump *keyretriever.Dump, b Browser, km KeyManager) {
	name := b.BrowserName()
	udd := b.UserDataDir()
	for i := range dump.Vaults {
		existing := &dump.Vaults[i]
		if existing.Browser == name && existing.UserDataDir == udd {
			existing.Profiles = append(existing.Profiles, b.ProfileName())
			return
		}
	}
	keys, err := km.ExportKeys()
	if err != nil {
		log.Warnf("dump-keys: %s/%s export failed: %v", name, b.ProfileName(), err)
		return
	}
	vault := keyretriever.Vault{
		Browser:     name,
		UserDataDir: udd,
		Profiles:    []string{b.ProfileName()},
		Keys:        keys,
	}
	dump.Vaults = append(dump.Vaults, vault)
}
