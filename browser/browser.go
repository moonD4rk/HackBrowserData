package browser

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/browser/safari"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser is one installation: a UserDataDir holding profiles that (for Chromium) share one master key.
type Browser interface {
	BrowserName() string
	UserDataDir() string
	Profiles() []types.Profile
	Extract(categories []types.Category) ([]types.ExtractResult, error)
	CountEntries(categories []types.Category) ([]types.CountResult, error)
}

type DiscoverOptions struct {
	Name             string // "all"|"chrome"|"firefox"|...
	ProfilePath      string // custom profile dir override
	KeychainPassword string // macOS only — see browser_darwin.go
}

// browserInjector injects decryption credentials into a Browser; built per-platform by newCredentialInjector.
type browserInjector func(Browser)

// DiscoverBrowsersWithKeys is DiscoverBrowsers plus credential injection, so the returned installations are ready for Extract.
// On macOS it may prompt for the login password — metadata-only callers should use DiscoverBrowsers to avoid the prompt.
func DiscoverBrowsersWithKeys(opts DiscoverOptions) ([]Browser, error) {
	browsers, err := DiscoverBrowsers(opts)
	if err != nil {
		return nil, err
	}
	inject := newCredentialInjector(opts)
	for _, b := range browsers {
		inject(b)
	}
	return browsers, nil
}

// DiscoverBrowsers skips credential injection: metadata (Profiles, CountEntries) works, Extract won't decrypt protected data,
// and macOS never prompts. Use it for list-style commands.
func DiscoverBrowsers(opts DiscoverOptions) ([]Browser, error) {
	return discoverFromConfigs(platformBrowsers(), opts)
}

// discoverFromConfigs is the testable core of DiscoverBrowsers; it deliberately does no credential injection.
func discoverFromConfigs(configs []types.BrowserConfig, opts DiscoverOptions) ([]Browser, error) {
	name := strings.ToLower(opts.Name)
	if name == "" {
		name = "all"
	}

	configs = resolveGlobs(configs)

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

		b, err := newBrowser(cfg)
		if err != nil {
			log.Errorf("browser %s: %v", cfg.Name, err)
			continue
		}
		if b == nil {
			log.Debugf("browser %s not found at %s", cfg.Name, cfg.UserDataDir)
			continue
		}

		browsers = append(browsers, b)
	}
	return browsers, nil
}

// KeyManager is implemented by installations accepting external master-key retrievers (Chromium only).
type KeyManager interface {
	SetRetrievers(masterkey.Retrievers)
	ExportKeys() (masterkey.MasterKeys, error)
}

// KeychainPasswordReceiver is implemented by installations that need the macOS login password (Safari only).
type KeychainPasswordReceiver interface {
	SetKeychainPassword(string)
}

// resolveGlobs expands UserDataDir glob patterns for Windows MSIX/UWP browsers whose package dirs carry a dynamic
// publisher-hash suffix (e.g. "TheBrowserCompany.Arc_*"). A glob matching N dirs yields N configs.
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

// newBrowser dispatches on BrowserKind, returning a nil Browser when no profile is found.
func newBrowser(cfg types.BrowserConfig) (Browser, error) {
	switch cfg.Kind {
	case types.Chromium, types.ChromiumYandex, types.ChromiumOpera:
		b, err := chromium.NewBrowser(cfg)
		if err != nil {
			return nil, err
		}
		if b == nil {
			return nil, nil
		}
		return b, nil

	case types.Firefox:
		b, err := firefox.NewBrowser(cfg)
		if err != nil {
			return nil, err
		}
		if b == nil {
			return nil, nil
		}
		return b, nil

	case types.Safari:
		b, err := safari.NewBrowser(cfg)
		if err != nil {
			return nil, err
		}
		if b == nil {
			return nil, nil
		}
		return b, nil

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
