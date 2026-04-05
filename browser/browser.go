package browser

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser is the interface that both chromium.Browser and firefox.Browser implement.
type Browser interface {
	BrowserName() string
	ProfileName() string
	ProfileDir() string
	Extract(categories []types.Category) (*types.BrowserData, error)
}

// PickOptions configures which browsers to pick.
type PickOptions struct {
	Name             string // browser name filter: "all"|"chrome"|"firefox"|...
	ProfilePath      string // custom profile directory override
	KeychainPassword string // macOS keychain password (ignored on other platforms)
}

// PickBrowsers returns browsers matching the given options.
// When Name is "all", all known browsers are tried.
// ProfilePath overrides the default user data directory (only when targeting a specific browser).
func PickBrowsers(opts PickOptions) ([]Browser, error) {
	return pickFromConfigs(platformBrowsers(), opts)
}

// pickFromConfigs is the testable core of PickBrowsers.
func pickFromConfigs(configs []types.BrowserConfig, opts PickOptions) ([]Browser, error) {
	name := strings.ToLower(opts.Name)
	if name == "" {
		name = "all"
	}

	var browsers []Browser
	for _, cfg := range configs {
		if name != "all" && cfg.Key != name {
			continue
		}

		if opts.ProfilePath != "" && name != "all" {
			if cfg.Kind == types.Firefox {
				cfg.UserDataDir = filepath.Dir(filepath.Clean(opts.ProfilePath))
			} else {
				cfg.UserDataDir = opts.ProfilePath
			}
		}

		if opts.KeychainPassword != "" {
			cfg.KeychainPassword = opts.KeychainPassword
		}

		bs, err := newBrowsers(cfg)
		if err != nil {
			log.Errorf("browser %s: %v", cfg.Name, err)
			continue
		}
		if len(bs) == 0 {
			log.Debugf("browser %s not found at %s", cfg.Name, cfg.UserDataDir)
			continue
		}
		browsers = append(browsers, bs...)
	}
	return browsers, nil
}

// newBrowsers dispatches to the correct engine based on BrowserKind.
func newBrowsers(cfg types.BrowserConfig) ([]Browser, error) {
	switch cfg.Kind {
	case types.Chromium, types.ChromiumYandex, types.ChromiumOpera:
		bs, err := chromium.NewBrowsers(cfg)
		if err != nil {
			return nil, err
		}
		browsers := make([]Browser, len(bs))
		for i, b := range bs {
			browsers[i] = b
		}
		return browsers, nil

	case types.Firefox:
		bs, err := firefox.NewBrowsers(cfg)
		if err != nil {
			return nil, err
		}
		browsers := make([]Browser, len(bs))
		for i, b := range bs {
			browsers[i] = b
		}
		return browsers, nil

	default:
		return nil, fmt.Errorf("unknown browser kind: %d", cfg.Kind)
	}
}

// ListBrowsers returns sorted keys of all platform browsers.
func ListBrowsers() []string {
	var l []string
	for _, cfg := range platformBrowsers() {
		l = append(l, cfg.Key)
	}
	sort.Strings(l)
	return l
}

// Names returns a pipe-separated list of browser keys for CLI help text.
func Names() string {
	return strings.Join(ListBrowsers(), "|")
}
