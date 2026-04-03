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
	Extract(categories []types.Category) (*types.BrowserData, error)
}

// PickBrowsers returns browsers matching the given name.
// When name is "all", all known browsers are tried.
// profilePath overrides the default user data directory (only when targeting a specific browser).
func PickBrowsers(name, profilePath string) ([]Browser, error) {
	return pickFromConfigs(platformBrowsers(), name, profilePath)
}

// pickFromConfigs is the testable core of PickBrowsers.
func pickFromConfigs(configs []types.BrowserConfig, name, profilePath string) ([]Browser, error) {
	name = strings.ToLower(name)

	var browsers []Browser
	for _, cfg := range configs {
		if name != "all" && cfg.Key != name {
			continue
		}

		if profilePath != "" && name != "all" {
			if cfg.Kind == types.KindFirefox {
				cfg.UserDataDir = filepath.Dir(filepath.Clean(profilePath))
			} else {
				cfg.UserDataDir = profilePath
			}
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
	case types.KindChromium, types.KindChromiumYandex, types.KindChromiumOpera:
		bs, err := chromium.NewBrowsers(cfg)
		if err != nil {
			return nil, err
		}
		browsers := make([]Browser, len(bs))
		for i, b := range bs {
			browsers[i] = b
		}
		return browsers, nil

	case types.KindFirefox:
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
