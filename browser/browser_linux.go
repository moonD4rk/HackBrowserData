//go:build linux

package browser

import (
	"github.com/moond4rk/hackbrowserdata/types"
)

func platformBrowsers() []types.BrowserConfig {
	return []types.BrowserConfig{
		{
			Key:         "chrome",
			Name:        chromeName,
			Kind:        types.KindChromium,
			Storage:     "Chrome Safe Storage",
			UserDataDir: homeDir + "/.config/google-chrome",
		},
		{
			Key:         "edge",
			Name:        edgeName,
			Kind:        types.KindChromium,
			Storage:     "Chromium Safe Storage",
			UserDataDir: homeDir + "/.config/microsoft-edge",
		},
		{
			Key:         "chromium",
			Name:        chromiumName,
			Kind:        types.KindChromium,
			Storage:     "Chromium Safe Storage",
			UserDataDir: homeDir + "/.config/chromium",
		},
		{
			Key:         "chrome-beta",
			Name:        chromeBetaName,
			Kind:        types.KindChromium,
			Storage:     "Chrome Safe Storage",
			UserDataDir: homeDir + "/.config/google-chrome-beta",
		},
		{
			Key:         "opera",
			Name:        operaName,
			Kind:        types.KindChromiumOpera,
			Storage:     "Chromium Safe Storage",
			UserDataDir: homeDir + "/.config/opera",
		},
		{
			Key:         "vivaldi",
			Name:        vivaldiName,
			Kind:        types.KindChromium,
			Storage:     "Chrome Safe Storage",
			UserDataDir: homeDir + "/.config/vivaldi",
		},
		{
			Key:         "brave",
			Name:        braveName,
			Kind:        types.KindChromium,
			Storage:     "Brave Safe Storage",
			UserDataDir: homeDir + "/.config/BraveSoftware/Brave-Browser",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.KindFirefox,
			UserDataDir: homeDir + "/.mozilla/firefox",
		},
	}
}
