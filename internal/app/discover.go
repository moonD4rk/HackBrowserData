package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

// Discovery keeps a failure per browser whose data could not be read, so it is reported, not taken as absence.
type Discovery struct {
	Dirs        []browser.UserDataDir
	DirFailures []DirFailure
}

func discover(ctx context.Context, browsers []browser.Browser, options browser.DiscoverOptions) Discovery {
	var discovery Discovery
	for _, targetBrowser := range browsers {
		dirs, err := discoverBrowser(ctx, targetBrowser, options)
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
			if hasData(dir, profile, sourcesFor(dir, profile)) {
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

func hasData(dir browser.UserDataDir, profile browser.Profile, sources map[category.Category][]browser.Source) bool {
	for _, categorySources := range sources {
		if len(categorySources) == 0 {
			continue
		}
		if _, ok, err := locate(dir, profile, categorySources); ok || err != nil {
			return true
		}
	}
	return false
}
