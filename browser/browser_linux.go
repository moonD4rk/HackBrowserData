//go:build linux

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
			Storage:     "Chrome Safe Storage",
			UserDataDir: homeDir + "/.config/google-chrome",
		},
		{
			Key:         "edge",
			Name:        edgeName,
			Kind:        types.Chromium,
			Storage:     "Chromium Safe Storage",
			UserDataDir: homeDir + "/.config/microsoft-edge",
		},
		{
			Key:         "chromium",
			Name:        chromiumName,
			Kind:        types.Chromium,
			Storage:     "Chromium Safe Storage",
			UserDataDir: homeDir + "/.config/chromium",
		},
		{
			Key:         "chrome-beta",
			Name:        chromeBetaName,
			Kind:        types.Chromium,
			Storage:     "Chrome Safe Storage",
			UserDataDir: homeDir + "/.config/google-chrome-beta",
		},
		{
			Key:         "opera",
			Name:        operaName,
			Kind:        types.ChromiumOpera,
			Storage:     "Chromium Safe Storage",
			UserDataDir: homeDir + "/.config/opera",
		},
		{
			Key:         "vivaldi",
			Name:        vivaldiName,
			Kind:        types.Chromium,
			Storage:     "Chrome Safe Storage",
			UserDataDir: homeDir + "/.config/vivaldi",
		},
		{
			Key:         "brave",
			Name:        braveName,
			Kind:        types.Chromium,
			Storage:     "Brave Safe Storage",
			UserDataDir: homeDir + "/.config/BraveSoftware/Brave-Browser",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.Firefox,
			UserDataDir: homeDir + "/.mozilla/firefox",
		},
	}
}

// newPlatformInjector returns a closure that wires the Linux Chromium master-key retrievers into
// each Browser. Linux has two tiers: V10 uses the "peanuts" hardcoded password (kV10Key); V11
// uses the D-Bus Secret Service keyring (kV11Key). V20 is nil — App-Bound Encryption is Windows-
// only. Both V10 and V11 run independently so a profile carrying mixed cipher prefixes decrypts
// both tiers.
func newPlatformInjector(_ PickOptions) func(Browser) {
	retrievers := keyretriever.DefaultRetrievers()
	return func(b Browser) {
		if s, ok := b.(keyRetrieversSetter); ok {
			s.SetKeyRetrievers(retrievers)
		}
	}
}
