package app

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
)

// Discovery keeps a failure per browser whose data could not be read, so it is reported, not taken as absence.
type Discovery struct {
	Dirs        []browser.UserDataDir
	DirFailures []DirFailure
}

// Discover applies the target's profile path only when one browser is named.
func Discover(target Target) Discovery {
	var discovery Discovery
	for _, supportedBrowser := range browser.Supported() {
		if target.BrowserName != allBrowsers && supportedBrowser.Name != target.BrowserName {
			continue
		}
		discoverOptions := browser.DiscoverOptions{HomeDir: target.HomeDir}
		if target.BrowserName != allBrowsers {
			discoverOptions.ProfilePath = target.ProfilePath
		}
		dirs, err := discover(supportedBrowser, discoverOptions)
		discovery.Dirs = append(discovery.Dirs, withData(dirs)...)
		if err != nil {
			discovery.DirFailures = append(discovery.DirFailures, DirFailure{UserDataDirID: browser.UserDataDirID(supportedBrowser.Name), Err: err})
		}
	}
	return discovery
}

// withData drops profiles holding no file any category reads, such as a bare Preferences, but keeps unreadable ones.
func withData(dirs []browser.UserDataDir) []browser.UserDataDir {
	extractors := extractorsWith(nil)
	var out []browser.UserDataDir
	for _, dir := range dirs {
		var profiles []browser.Profile
		for _, profile := range dir.Profiles {
			if hasData(dir, profile, extractors(dir, profile)) {
				profiles = append(profiles, profile)
			}
		}
		if len(profiles) > 0 {
			dir.Profiles = profiles
			out = append(out, dir)
		}
	}
	return out
}

func hasData(dir browser.UserDataDir, profile browser.Profile, extractors browser.Extractors) bool {
	for _, extractor := range extractors {
		if len(extractor.Sources) == 0 {
			continue
		}
		if _, ok, err := locate(dir, profile, extractor.Sources); ok || err != nil {
			return true
		}
	}
	return false
}

// discover dispatches on Base here, not in browser, because the base packages import browser.
func discover(supportedBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	switch supportedBrowser.Base {
	case browser.Chromium:
		return chromium.Discover(supportedBrowser, options)
	case browser.Firefox:
		return firefox.Discover(supportedBrowser, options)
	case browser.Safari:
		return safari.Discover(supportedBrowser, options)
	default:
		return nil, fmt.Errorf("%s: no base package", supportedBrowser.Name)
	}
}

// extractorsWith hands Safari the run's login keychain, which its default profile's password extractor reads.
func extractorsWith(keychain safari.LoginKeychain) extractorsFunc {
	return func(dir browser.UserDataDir, profile browser.Profile) browser.Extractors {
		switch dir.Browser.Base {
		case browser.Chromium:
			return chromium.Extractors(dir, profile)
		case browser.Firefox:
			return firefox.Extractors(dir, profile)
		case browser.Safari:
			return safari.Extractors(dir, profile, keychain)
		default:
			return nil
		}
	}
}
