//go:build windows

package browser

import (
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/types"
)

func platformBrowsers() []types.BrowserConfig {
	return []types.BrowserConfig{
		{
			Key:         "chrome",
			Name:        chromeName,
			Kind:        types.Chromium,
			Storage:     "chrome",
			UserDataDir: homeDir + "/AppData/Local/Google/Chrome/User Data",
		},
		{
			Key:         "edge",
			Name:        edgeName,
			Kind:        types.Chromium,
			Storage:     "edge",
			UserDataDir: homeDir + "/AppData/Local/Microsoft/Edge/User Data",
		},
		{
			Key:         "chromium",
			Name:        chromiumName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/Chromium/User Data",
		},
		{
			Key:         "chrome-beta",
			Name:        chromeBetaName,
			Kind:        types.Chromium,
			Storage:     "chrome-beta",
			UserDataDir: homeDir + "/AppData/Local/Google/Chrome Beta/User Data",
		},
		{
			Key:         "opera",
			Name:        operaName,
			Kind:        types.ChromiumOpera,
			UserDataDir: homeDir + "/AppData/Roaming/Opera Software/Opera Stable",
		},
		{
			Key:         "opera-gx",
			Name:        operaGXName,
			Kind:        types.ChromiumOpera,
			UserDataDir: homeDir + "/AppData/Roaming/Opera Software/Opera GX Stable",
		},
		{
			Key:         "vivaldi",
			Name:        vivaldiName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/Vivaldi/User Data",
		},
		{
			Key:         "coccoc",
			Name:        coccocName,
			Kind:        types.Chromium,
			Storage:     "coccoc",
			UserDataDir: homeDir + "/AppData/Local/CocCoc/Browser/User Data",
		},
		{
			Key:         "brave",
			Name:        braveName,
			Kind:        types.Chromium,
			Storage:     "brave",
			UserDataDir: homeDir + "/AppData/Local/BraveSoftware/Brave-Browser/User Data",
		},
		{
			Key:         "yandex",
			Name:        yandexName,
			Kind:        types.ChromiumYandex,
			UserDataDir: homeDir + "/AppData/Local/Yandex/YandexBrowser/User Data",
		},
		{
			Key:         "360x",
			Name:        speed360XName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/360ChromeX/Chrome/User Data",
		},
		{
			Key:         "360",
			Name:        speed360Name,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/360chrome/Chrome/User Data",
		},
		{
			Key:         "qq",
			Name:        qqName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/Tencent/QQBrowser/User Data",
		},
		{
			Key:         "dc",
			Name:        dcName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/DCBrowser/User Data",
		},
		{
			Key:         "sogou",
			Name:        sogouName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/Sogou/SogouExplorer/User Data",
		},
		{
			Key:         "arc",
			Name:        arcName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/Packages/TheBrowserCompany.Arc_*/LocalCache/Local/Arc/User Data",
		},
		{
			Key:         "duckduckgo",
			Name:        duckduckgoName,
			Kind:        types.Chromium,
			UserDataDir: homeDir + "/AppData/Local/Packages/DuckDuckGo.DesktopBrowser_*/LocalState/EBWebView",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.Firefox,
			UserDataDir: homeDir + "/AppData/Roaming/Mozilla/Firefox/Profiles",
		},
	}
}

// newPlatformInjector returns a closure that wires the Windows v10 (DPAPI) and v20 (ABE) Chromium
// master-key retrievers into each Browser. Per issue #578 the two tiers are orthogonal — a single
// Chrome profile upgraded from pre-127 carries v20 cookies alongside v10 passwords — so both
// retrievers run independently rather than as a first-success chain.
func newPlatformInjector(_ PickOptions) func(Browser) {
	retrievers := keyretriever.DefaultRetrievers()
	return func(b Browser) {
		if s, ok := b.(keyRetrieversSetter); ok {
			s.SetKeyRetrievers(retrievers)
		}
	}
}
