package app

import (
	"context"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
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
		discovery.Dirs = append(discovery.Dirs, dropEmptyProfiles(dirs)...)
		if err != nil {
			discovery.DirFailures = append(discovery.DirFailures, DirFailure{DirID: targetBrowser.UserDataDirID(0), Err: err})
		}
	}
	return discovery
}

// dropEmptyProfiles drops profiles holding no file any category reads, such as a bare Preferences, and a dir left with
// none. It keeps an unreadable profile, so the failure is reported.
func dropEmptyProfiles(dirs []browser.UserDataDir) []browser.UserDataDir {
	var kept []browser.UserDataDir
	for _, dir := range dirs {
		var profiles []browser.Profile
		for _, profile := range dir.Profiles {
			if hasData(dir, profile) {
				profiles = append(profiles, profile)
			}
		}
		if len(profiles) > 0 {
			dir.Profiles = profiles
			kept = append(kept, dir)
		}
	}
	return kept
}

func hasData(dir browser.UserDataDir, profile browser.Profile) bool {
	for _, categorySources := range profileSources(dir, profile) {
		if len(categorySources) == 0 {
			continue
		}
		if _, ok, err := locate(dir, profile, categorySources); ok || err != nil {
			return true
		}
	}
	return false
}
