package app

import (
	"context"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/internal/browser/safari"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// discoverBrowser, like every function in this file, dispatches on Base here because the base packages import browser.
func discoverBrowser(ctx context.Context, targetBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
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

func profileSources(dir browser.UserDataDir, profile browser.Profile) map[category.Category][]browser.Source {
	switch dir.Browser.Base {
	case browser.Chromium:
		return chromium.Sources(dir, profile)
	case browser.Firefox:
		return firefox.Sources(dir, profile)
	case browser.Safari:
		return safari.Sources(dir, profile)
	default:
		return nil
	}
}

// needsMasterKeys is false outside Chromium, the one base whose dir holds master keys.
func needsMasterKeys(dir browser.UserDataDir, extractions []Extraction) bool {
	if dir.Browser.Base != browser.Chromium {
		return false
	}
	for _, extraction := range extractions {
		if chromium.NeedsMasterKeys(dir, extraction.Category) {
			return true
		}
	}
	return false
}

// extract hands each base its own secret: Chromium the dir's master keys, Firefox the primary password, and Safari the
// login keychain.
func (r runner) extract(
	ctx context.Context, captured browser.ProfileSnapshot, masterKeys masterkey.Set, dataCategory category.Category, out browser.Collector,
) error {
	switch captured.UserDataDir.Browser.Base {
	case browser.Chromium:
		return chromium.Extract(ctx, captured, masterKeys, dataCategory, out)
	case browser.Firefox:
		return firefox.Extract(ctx, captured, r.firefoxPrimaryPassword, dataCategory, out)
	case browser.Safari:
		return safari.Extract(ctx, captured, r.loginKeychain, dataCategory, out)
	default:
		return fmt.Errorf("%s: no base package", captured.UserDataDir.Browser.Name)
	}
}
