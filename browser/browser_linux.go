//go:build linux

package browser

import (
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

func platformBrowsers() []types.BrowserConfig {
	return []types.BrowserConfig{
		{
			Key:           "chrome",
			Name:          chromeName,
			Kind:          types.Chromium,
			KeychainLabel: "Chrome Safe Storage",
			UserDataDir:   homeDir + "/.config/google-chrome",
		},
		{
			Key:           "edge",
			Name:          edgeName,
			Kind:          types.Chromium,
			KeychainLabel: "Chromium Safe Storage",
			UserDataDir:   homeDir + "/.config/microsoft-edge",
		},
		{
			Key:           "chromium",
			Name:          chromiumName,
			Kind:          types.Chromium,
			KeychainLabel: "Chromium Safe Storage",
			UserDataDir:   homeDir + "/.config/chromium",
		},
		{
			Key:           "chrome-beta",
			Name:          chromeBetaName,
			Kind:          types.Chromium,
			KeychainLabel: "Chrome Safe Storage",
			UserDataDir:   homeDir + "/.config/google-chrome-beta",
		},
		{
			Key:           "opera",
			Name:          operaName,
			Kind:          types.ChromiumOpera,
			KeychainLabel: "Chromium Safe Storage",
			UserDataDir:   homeDir + "/.config/opera",
		},
		{
			Key:           "vivaldi",
			Name:          vivaldiName,
			Kind:          types.Chromium,
			KeychainLabel: "Chrome Safe Storage",
			UserDataDir:   homeDir + "/.config/vivaldi",
		},
		{
			Key:           "brave",
			Name:          braveName,
			Kind:          types.Chromium,
			KeychainLabel: "Brave Safe Storage",
			UserDataDir:   homeDir + "/.config/BraveSoftware/Brave-Browser",
		},
		{
			Key:         "firefox",
			Name:        firefoxName,
			Kind:        types.Firefox,
			UserDataDir: homeDir + "/.mozilla/firefox",
		},
	}
}

// newCredentialInjector wires the Linux Chromium retrievers: V10 ("peanuts" hardcoded) and V11 (D-Bus Secret Service),
// run independently for mixed-cipher profiles. V20 is nil — App-Bound Encryption is Windows-only.
func newCredentialInjector(_ DiscoverOptions) browserInjector {
	retrievers := masterkey.DefaultRetrievers()
	return func(b Browser) {
		if km, ok := b.(KeyManager); ok {
			km.SetRetrievers(retrievers)
		}
	}
}
