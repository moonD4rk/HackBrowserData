package browser

import (
	"os"

	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

var homeDir, _ = os.UserHomeDir()

// platformBrowsers adapts this OS's rows of the browser table; joining with "/" keeps today's user data dir strings.
func platformBrowsers() []types.BrowserConfig {
	var configs []types.BrowserConfig
	for _, d := range ibrowser.Supported() {
		p := d.Platform()
		for _, dir := range p.UserDataDirs {
			configs = append(configs, types.BrowserConfig{
				Key:           d.ID,
				Name:          d.Name,
				Kind:          kindOf(d),
				KeychainLabel: p.KeychainLabel,
				WindowsABE:    p.ABE != masterkey.ABENone,
				UserDataDir:   homeDir + "/" + dir,
			})
		}
	}
	return configs
}

// kindOf derives today's engine kind; Yandex and the Opera family are the forks with their own on-disk format.
func kindOf(d ibrowser.Def) types.BrowserKind {
	switch {
	case d.Base == ibrowser.Firefox:
		return types.Firefox
	case d.Base == ibrowser.Safari:
		return types.Safari
	case d.ID == "yandex":
		return types.ChromiumYandex
	case d.ID == "opera", d.ID == "opera-gx", d.ID == "vought":
		return types.ChromiumOpera
	default:
		return types.Chromium
	}
}
