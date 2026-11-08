package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
)

// A vault names its on-disk format by kind, so a reader without the browser in its table still picks its files.
const (
	kindChromium = "chromium"
	kindYandex   = "chromium-yandex"
	kindOpera    = "chromium-opera"
)

// kinds holds the forks of overrides; every other Chromium browser is kindChromium.
var kinds = map[string]string{
	"yandex":   kindYandex,
	"opera":    kindOpera,
	"opera-gx": kindOpera,
	"vought":   kindOpera,
}

func VaultKind(chromiumBrowser browser.Browser) string {
	if kind, ok := kinds[chromiumBrowser.Name]; ok {
		return kind
	}
	return kindChromium
}

// BrowserNameForKind names the browser whose extractors read a vault of kind; a plain Chromium vault keeps its own.
func BrowserNameForKind(kind, name string) (string, error) {
	switch kind {
	case kindChromium:
		return name, nil
	case kindYandex:
		return "yandex", nil
	case kindOpera:
		return "opera", nil
	default:
		return "", fmt.Errorf("unknown vault kind %q", kind)
	}
}
