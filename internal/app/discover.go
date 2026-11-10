package app

import (
	"context"
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

func Discover(ctx context.Context, target Target) Discovery {
	var discovery Discovery
	discoverOptions := browser.DiscoverOptions{HomeDir: target.HomeDir, ProfilePath: target.ProfilePath}
	for _, targetBrowser := range target.Browsers {
		dirs, err := discover(ctx, targetBrowser, discoverOptions)
		discovery.Dirs = append(discovery.Dirs, withData(dirs)...)
		if err != nil {
			discovery.DirFailures = append(discovery.DirFailures, DirFailure{UserDataDirID: browser.UserDataDirID(targetBrowser.Name), Err: err})
		}
	}
	return discovery
}

// withData drops profiles holding no file any category reads, such as a bare Preferences, but keeps unreadable ones.
func withData(dirs []browser.UserDataDir) []browser.UserDataDir {
	var out []browser.UserDataDir
	for _, dir := range dirs {
		var profiles []browser.Profile
		for _, profile := range dir.Profiles {
			if hasData(dir, profile, extractorsFor(dir, profile)) {
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
func discover(ctx context.Context, targetBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	switch targetBrowser.Base {
	case browser.Chromium:
		return chromium.Discover(targetBrowser, options)
	case browser.Firefox:
		return firefox.Discover(targetBrowser, options)
	case browser.Safari:
		return safari.Discover(ctx, targetBrowser, options)
	default:
		return nil, fmt.Errorf("%s: no base package", targetBrowser.Name)
	}
}

// extractorsFor dispatches on Base here for the same reason discover does.
func extractorsFor(dir browser.UserDataDir, profile browser.Profile) browser.Extractors {
	switch dir.Browser.Base {
	case browser.Chromium:
		return chromium.Extractors(dir, profile)
	case browser.Firefox:
		return firefox.Extractors(dir, profile)
	case browser.Safari:
		return safari.Extractors(dir, profile)
	default:
		return nil
	}
}
