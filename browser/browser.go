package browser

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/browser/safari"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser is the interface implemented by every engine package —
// chromium.Browser, firefox.Browser, and safari.Browser.
type Browser interface {
	BrowserName() string
	ProfileName() string
	ProfileDir() string
	Extract(categories []types.Category) (*types.BrowserData, error)
	CountEntries(categories []types.Category) (map[types.Category]int, error)
}

// PickOptions configures which browsers to pick.
type PickOptions struct {
	Name             string // browser name filter: "all"|"chrome"|"firefox"|...
	ProfilePath      string // custom profile directory override
	KeychainPassword string // macOS only — see browser_darwin.go
}

// PickBrowsers returns browsers matching the given options.
// When Name is "all", all known browsers are tried.
// ProfilePath overrides the default user data directory (only when targeting a specific browser).
func PickBrowsers(opts PickOptions) ([]Browser, error) {
	return pickFromConfigs(platformBrowsers(), opts)
}

// pickFromConfigs is the testable core of PickBrowsers: it discovers
// installed profiles and wires each Browser up via newPlatformInjector.
func pickFromConfigs(configs []types.BrowserConfig, opts PickOptions) ([]Browser, error) {
	name := strings.ToLower(opts.Name)
	if name == "" {
		name = "all"
	}

	inject := newPlatformInjector(opts)
	configs = resolveGlobs(configs)

	var browsers []Browser
	for _, cfg := range configs {
		if name != "all" && cfg.Key != name {
			continue
		}

		// Override profile directory when targeting a specific browser.
		if opts.ProfilePath != "" && name != "all" {
			if cfg.Kind == types.Firefox {
				cfg.UserDataDir = filepath.Dir(filepath.Clean(opts.ProfilePath))
			} else {
				cfg.UserDataDir = opts.ProfilePath
			}
		}

		found, err := newBrowsers(cfg)
		if err != nil {
			log.Errorf("browser %s: %v", cfg.Name, err)
			continue
		}
		if len(found) == 0 {
			log.Debugf("browser %s not found at %s", cfg.Name, cfg.UserDataDir)
			continue
		}

		for _, b := range found {
			inject(b)
		}
		browsers = append(browsers, found...)
	}
	return browsers, nil
}

// retrieverSetter is an optional capability interface. Chromium variants
// implement it to receive a master-key retriever chain; Firefox and Safari
// do not.
type retrieverSetter interface {
	SetRetriever(keyretriever.KeyRetriever)
}

// resolveGlobs expands glob patterns in browser configs' UserDataDir.
// This supports MSIX/UWP browsers on Windows whose package directories
// contain a dynamic publisher hash suffix (e.g., "TheBrowserCompany.Arc_*").
//
// For literal paths (no glob metacharacters), Glob returns the path itself
// when it exists, so the config passes through unchanged. When a path does
// not exist and contains no metacharacters, Glob returns nil and the
// original config is preserved — the main loop handles "not found" as usual.
//
// When a glob matches multiple directories, the config is duplicated so
// each resolved path is treated as a separate browser data directory.
func resolveGlobs(configs []types.BrowserConfig) []types.BrowserConfig {
	var out []types.BrowserConfig
	for _, cfg := range configs {
		matches, _ := filepath.Glob(cfg.UserDataDir)
		if len(matches) == 0 {
			out = append(out, cfg)
			continue
		}
		for _, dir := range matches {
			c := cfg
			c.UserDataDir = dir
			out = append(out, c)
		}
	}
	return out
}

// newBrowsers dispatches to the correct engine based on BrowserKind
// and converts engine-specific types to the Browser interface.
func newBrowsers(cfg types.BrowserConfig) ([]Browser, error) {
	switch cfg.Kind {
	case types.Chromium, types.ChromiumYandex, types.ChromiumOpera:
		found, err := chromium.NewBrowsers(cfg)
		if err != nil {
			return nil, err
		}
		result := make([]Browser, len(found))
		for i, b := range found {
			result[i] = b
		}
		return result, nil

	case types.Firefox:
		found, err := firefox.NewBrowsers(cfg)
		if err != nil {
			return nil, err
		}
		result := make([]Browser, len(found))
		for i, b := range found {
			result[i] = b
		}
		return result, nil

	case types.Safari:
		found, err := safari.NewBrowsers(cfg)
		if err != nil {
			return nil, err
		}
		result := make([]Browser, len(found))
		for i, b := range found {
			result[i] = b
		}
		return result, nil

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
