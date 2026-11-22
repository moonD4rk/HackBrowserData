package chromium

import (
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

// vaultKindChromium is the keys.json kind of every Chromium browser no override covers.
const vaultKindChromium = "chromium"

// override is a fork that stores data its own way: its browsers, its keys.json kind, the extractors it swaps, and the
// categories its files do not hold.
type override struct {
	browserNames []string
	vaultKind    string // "" keeps vaultKindChromium
	extractors   map[category.Category]extractor
	unsupported  []category.Category
}

// overrides match Browser.Name, never UserDataDirID, which differs for a second user data dir of one browser.
var overrides = []override{
	{
		browserNames: []string{"yandex"},
		vaultKind:    "chromium-yandex",
		extractors: map[category.Category]extractor{
			category.Password:   newDecryptingExtractor(extractYandexPasswords, yaPassmanData),
			category.CreditCard: newDecryptingExtractor(extractYandexCreditCards, yaCreditCards),
		},
	},
	{
		browserNames: []string{"opera", "opera-gx", "vought"},
		vaultKind:    "chromium-opera",
		extractors: map[category.Category]extractor{
			category.Extension: newExtensionExtractor("extensions.opsettings"),
		},
	},
	{
		// QQ and Sogou keep no extension list in their Preferences files.
		browserNames: []string{"qq", "sogou"},
		unsupported:  []category.Category{category.Extension},
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
		if fork.vaultKind != "" && fork.vaultKind == vaultKind {
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
	if fork, ok := overrideFor(chromiumBrowser.Name); ok && fork.vaultKind != "" {
		return fork.vaultKind
	}
	return vaultKindChromium
}

// overrideForDir matches the browser's name first; a restored browser this build does not know matches by its kind.
func overrideForDir(dir browser.UserDataDir) (override, bool) {
	if fork, ok := overrideFor(dir.Browser.Name); ok {
		return fork, true
	}
	return overrideForVaultKind(dir.VaultKind)
}
