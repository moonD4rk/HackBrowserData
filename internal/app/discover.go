package app

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
)

// Discovery is what Discover found: the user data dirs, and an error result for each browser whose data could not be
// read, which is reported rather than mistaken for absence.
type Discovery struct {
	Dirs   []browser.UserDataDir
	Failed []Result
}

// Discover looks for the requested browsers on this machine. -p applies only when one browser is named.
func Discover(req Request) Discovery {
	var d Discovery
	for _, def := range browser.Supported() {
		if req.Browser != allBrowsers && def.ID != req.Browser {
			continue
		}
		opts := browser.DiscoverOptions{Home: req.Home}
		if req.Browser != allBrowsers {
			opts.ProfilePath = req.ProfilePath
		}
		dirs, err := discover(def, opts)
		d.Dirs = append(d.Dirs, withData(dirs)...)
		if err != nil {
			d.Failed = append(d.Failed, dirFailure(browser.DirID(def.ID), err))
		}
	}
	return d
}

// withData keeps the profiles holding a file some category reads, and the user data dirs with such a profile: a
// Preferences marker alone is no data, so a profile created and never used is not listed, archived, or keyed. A
// profile whose files cannot be checked stays, so the plan reports the error.
func withData(dirs []browser.UserDataDir) []browser.UserDataDir {
	extractors := extractorsWith(nil)
	var out []browser.UserDataDir
	for _, dir := range dirs {
		var profiles []browser.Profile
		for _, p := range dir.Profiles {
			if hasData(dir, p, extractors(dir, p)) {
				profiles = append(profiles, p)
			}
		}
		if len(profiles) > 0 {
			dir.Profiles = profiles
			out = append(out, dir)
		}
	}
	return out
}

func hasData(dir browser.UserDataDir, p browser.Profile, ex browser.Extractors) bool {
	for _, x := range ex {
		if len(x.Sources) == 0 {
			continue
		}
		if _, ok, err := locate(dir, p, x.Sources); ok || err != nil {
			return true
		}
	}
	return false
}

// discover and extractorsWith dispatch on the base here, because the base packages import browser and so browser
// cannot call them.
func discover(def browser.Def, opts browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	switch def.Base {
	case browser.Chromium:
		return chromium.Discover(def, opts)
	case browser.Firefox:
		return firefox.Discover(def, opts)
	case browser.Safari:
		return safari.Discover(def, opts)
	default:
		return nil, fmt.Errorf("%s: no base package", def.ID)
	}
}

// extractorsWith hands Safari the run's login keychain, which its default profile's password extractor reads.
func extractorsWith(kc safari.LoginKeychain) extractorsFunc {
	return func(dir browser.UserDataDir, p browser.Profile) browser.Extractors {
		switch dir.Def.Base {
		case browser.Chromium:
			return chromium.Extractors(dir, p)
		case browser.Firefox:
			return firefox.Extractors(dir, p)
		case browser.Safari:
			return safari.Extractors(dir, p, kc)
		default:
			return nil
		}
	}
}
