package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
)

// A keys.json vault names its on-disk format by kind, so a reader without the browser in its table still picks the
// forks' own files.
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

// VaultKind is def's kind in keys.json.
func VaultKind(def browser.Def) string {
	if kind, ok := kinds[def.ID]; ok {
		return kind
	}
	return kindChromium
}

// FormatID is the Def.ID whose extractors read a vault of kind; a plain Chromium vault keeps its own id.
func FormatID(kind, id string) (string, error) {
	switch kind {
	case kindChromium:
		return id, nil
	case kindYandex:
		return "yandex", nil
	case kindOpera:
		return "opera", nil
	default:
		return "", fmt.Errorf("unknown engine kind %q", kind)
	}
}
