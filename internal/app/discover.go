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
		if req.Browser != "all" && def.ID != req.Browser {
			continue
		}
		opts := browser.DiscoverOptions{Home: req.Home}
		if req.Browser != "all" {
			opts.ProfilePath = req.ProfilePath
		}
		dirs, err := discover(def, opts)
		d.Dirs = append(d.Dirs, dirs...)
		if err != nil {
			d.Failed = append(d.Failed, dirFailure(browser.DirID(def.ID), err))
		}
	}
	return d
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
