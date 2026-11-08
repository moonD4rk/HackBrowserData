package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

// kindChromium is the keys.json kind of every Chromium browser no override covers.
const kindChromium = "chromium"

// override is a fork that stores data its own way: its browsers, its keys.json kind, and the extractors it swaps.
type override struct {
	browserNames []string
	kind         string
	extractors   browser.Extractors
}

// overrides match Browser.Name, never UserDataDirID, which differs for a second user data dir of one browser.
var overrides = []override{
	{
		browserNames: []string{"yandex"},
		kind:         "chromium-yandex",
		extractors: browser.Extractors{
			category.Password:   newDecryptingExtractor(newFileSpec("Ya Passman Data"), extractYandexPasswords),
			category.CreditCard: newDecryptingExtractor(newFileSpec("Ya Credit Cards"), extractYandexCreditCards),
		},
	},
	{
		browserNames: []string{"opera", "opera-gx", "vought"},
		kind:         "chromium-opera",
		extractors: browser.Extractors{
			category.Extension: newPlainExtractor(newFileSpec("Secure Preferences"), extractOperaExtensions),
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

// VaultKind names a vault's on-disk format, so a reader without the browser in its table still picks its files.
func VaultKind(chromiumBrowser browser.Browser) string {
	if fork, ok := overrideFor(chromiumBrowser.Name); ok {
		return fork.kind
	}
	return kindChromium
}

// BrowserNameForKind names the browser whose extractors read a vault of kind; a plain Chromium vault keeps its own.
func BrowserNameForKind(kind, name string) (string, error) {
	if kind == kindChromium {
		return name, nil
	}
	for _, fork := range overrides {
		if fork.kind == kind {
			return fork.browserNames[0], nil
		}
	}
	return "", fmt.Errorf("unknown vault kind %q", kind)
}
