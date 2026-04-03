//go:build darwin

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
			Storage:     "Chrome",
			UserDataDir: homeDir + "/Library/Application Support/Google/Chrome",
		},
		{
			Key:         "edge",
			Name:        edgeName,
			Kind:        types.KindChromium,
			Storage:     "Microsoft Edge",
			UserDataDir: homeDir + "/Library/Application Support/Microsoft Edge",
		},
		{
			Key:         "chromium",
			Name:        chromiumName,
			Kind:        types.KindChromium,
			Storage:     "Chromium",
			UserDataDir: homeDir + "/Library/Application Support/Chromium",
		},
		{
			Key:         "chrome-beta",
			Name:        chromeBetaName,
			Kind:        types.KindChromium,
			Storage:     "Chrome",
			UserDataDir: homeDir + "/Library/Application Support/Google/Chrome Beta",
		},
		{
			Key:         "opera",
			Name:        operaName,
			Kind:        types.KindChromiumOpera,
			Storage:     "Opera",
			UserDataDir: homeDir + "/Library/Application Support/com.operasoftware.Opera",
		},
		{
			Key:         "opera-gx",
			Name:        operaGXName,
			Kind:        types.KindChromiumOpera,
			Storage:     "Opera",
			UserDataDir: homeDir + "/Library/Application Support/com.operasoftware.OperaGX",
		},
		{
			Key:         "vivaldi",
			Name:        vivaldiName,
			Kind:        types.KindChromium,
			Storage:     "Vivaldi",
			UserDataDir: homeDir + "/Library/Application Support/Vivaldi",
		},
		{
			Key:         "coccoc",
			Name:        coccocName,
			Kind:        types.KindChromium,
			Storage:     "CocCoc",
			UserDataDir: homeDir + "/Library/Application Support/Coccoc",
		},
		{
			Key:         "brave",
			Name:        braveName,
			Kind:        types.KindChromium,
			Storage:     "Brave",
			UserDataDir: homeDir + "/Library/Application Support/BraveSoftware/Brave-Browser",
		},
		{
			Key:         "yandex",
			Name:        yandexName,
			Kind:        types.KindChromiumYandex,
			Storage:     "Yandex",
			UserDataDir: homeDir + "/Library/Application Support/Yandex/YandexBrowser",
		},
		{
			Key:         "arc",
			Name:        arcName,
			Kind:        types.KindChromium,
			Storage:     "Arc",
			UserDataDir: homeDir + "/Library/Application Support/Arc/User Data",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.KindFirefox,
			UserDataDir: homeDir + "/Library/Application Support/Firefox/Profiles",
		},
	}
}
