//go:build windows

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
			UserDataDir: homeDir + "/AppData/Local/Google/Chrome/User Data",
		},
		{
			Key:         "edge",
			Name:        edgeName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/Microsoft/Edge/User Data",
		},
		{
			Key:         "chromium",
			Name:        chromiumName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/Chromium/User Data",
		},
		{
			Key:         "chrome-beta",
			Name:        chromeBetaName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/Google/Chrome Beta/User Data",
		},
		{
			Key:         "opera",
			Name:        operaName,
			Kind:        types.KindChromiumOpera,
			UserDataDir: homeDir + "/AppData/Roaming/Opera Software/Opera Stable",
		},
		{
			Key:         "opera-gx",
			Name:        operaGXName,
			Kind:        types.KindChromiumOpera,
			UserDataDir: homeDir + "/AppData/Roaming/Opera Software/Opera GX Stable",
		},
		{
			Key:         "vivaldi",
			Name:        vivaldiName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/Vivaldi/User Data",
		},
		{
			Key:         "coccoc",
			Name:        coccocName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/CocCoc/Browser/User Data",
		},
		{
			Key:         "brave",
			Name:        braveName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/BraveSoftware/Brave-Browser/User Data",
		},
		{
			Key:         "yandex",
			Name:        yandexName,
			Kind:        types.KindChromiumYandex,
			UserDataDir: homeDir + "/AppData/Local/Yandex/YandexBrowser/User Data",
		},
		{
			Key:         "360x",
			Name:        speed360XName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/360ChromeX/Chrome/User Data",
		},
		{
			Key:         "360",
			Name:        speed360Name,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/360chrome/Chrome/User Data",
		},
		{
			Key:         "qq",
			Name:        qqBrowserName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/Tencent/QQBrowser/User Data",
		},
		{
			Key:         "dc",
			Name:        dcBrowserName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/DCBrowser/User Data",
		},
		{
			Key:         "sogou",
			Name:        sogouName,
			Kind:        types.KindChromium,
			UserDataDir: homeDir + "/AppData/Local/Sogou/SogouExplorer/User Data",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.KindFirefox,
			UserDataDir: homeDir + "/AppData/Roaming/Mozilla/Firefox/Profiles",
		},
	}
}
