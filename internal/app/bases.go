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

// bases is how a run reaches the base packages: browserBases in every command, a fake in the tests.
type bases interface {
	sources(dir browser.UserDataDir, profile browser.Profile) map[category.Category][]browser.Source
	extract(
		ctx context.Context, captured browser.ProfileSnapshot, masterKeys masterkey.Set, dataCategory category.Category, out browser.Collector,
	) error
}

// browserBases hands each base its own secret: Chromium the dir's master keys, Firefox the primary password, and
// Safari the login keychain.
type browserBases struct {
	firefoxPrimaryPassword string
	loginKeychain          safari.LoginKeychain
}

func (browserBases) sources(dir browser.UserDataDir, profile browser.Profile) map[category.Category][]browser.Source {
	return sourcesFor(dir, profile)
}

func (b browserBases) extract(
	ctx context.Context, captured browser.ProfileSnapshot, masterKeys masterkey.Set, dataCategory category.Category, out browser.Collector,
) error {
	switch captured.UserDataDir.Browser.Base {
	case browser.Chromium:
		return chromium.Extract(ctx, captured, masterKeys, dataCategory, out)
	case browser.Firefox:
		return firefox.Extract(ctx, captured, b.firefoxPrimaryPassword, dataCategory, out)
	case browser.Safari:
		return safari.Extract(ctx, captured, b.loginKeychain, dataCategory, out)
	default:
		return fmt.Errorf("%s: no base package", captured.UserDataDir.Browser.Name)
	}
}

// discoverBrowser dispatches on Base here, not in browser, because the base packages import browser.
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

// sourcesFor dispatches on Base for the same reason discoverBrowser does.
func sourcesFor(dir browser.UserDataDir, profile browser.Profile) map[category.Category][]browser.Source {
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

// needsMasterKeys dispatches on Base for the same reason; Chromium is the one base whose dir holds master keys.
func needsMasterKeys(dir browser.UserDataDir, dataCategory category.Category) bool {
	return dir.Browser.Base == browser.Chromium && chromium.NeedsMasterKeys(dataCategory)
}
