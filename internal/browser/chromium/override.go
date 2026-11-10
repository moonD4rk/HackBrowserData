package chromium

import (
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

// vaultKindChromium is the keys.json kind of every Chromium browser no override covers.
const vaultKindChromium = "chromium"

// override is a fork that stores data its own way: its browsers, its keys.json kind, and the extractors it swaps.
type override struct {
	browserNames []string
	vaultKind    string
	extractors   browser.Extractors
}

// overrides match Browser.Name, never UserDataDirID, which differs for a second user data dir of one browser.
var overrides = []override{
	{
		browserNames: []string{"yandex"},
		vaultKind:    "chromium-yandex",
		extractors: browser.Extractors{
			category.Password:   newDecryptingExtractor(extractYandexPasswords, yaPassmanData),
			category.CreditCard: newDecryptingExtractor(extractYandexCreditCards, yaCreditCards),
		},
	},
	{
		browserNames: []string{"opera", "opera-gx", "vought"},
		vaultKind:    "chromium-opera",
		extractors: browser.Extractors{
			category.Extension: newPlainExtractor(extractOperaExtensions, securePreferences),
		},
	},
}

func overrideFor(browserName string) (override, bool) {
	for _, fork := range overrides {
		for _, name := range fork.browserNames {
			if name == browserName {
				return fork, true
			}
		}
	}
	return override{}, false
}

func overrideForVaultKind(vaultKind string) (override, bool) {
	for _, fork := range overrides {
		if fork.vaultKind == vaultKind {
			return fork, true
		}
	}
	return override{}, false
}

func isVaultKind(vaultKind string) bool {
	_, ok := overrideForVaultKind(vaultKind)
	return ok || vaultKind == vaultKindChromium
}

// VaultKind names a vault's on-disk format, so a reader without the browser in its table still picks its files.
func VaultKind(chromiumBrowser browser.Browser) string {
	if fork, ok := overrideFor(chromiumBrowser.Name); ok {
		return fork.vaultKind
	}
	return vaultKindChromium
}

// vaultKindOf is a restored dir's own kind, or else its browser's.
func vaultKindOf(dir browser.UserDataDir) string {
	if dir.VaultKind != "" {
		return dir.VaultKind
	}
	return VaultKind(dir.Browser)
}
