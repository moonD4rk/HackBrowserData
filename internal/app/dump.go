package app

import (
	"context"
	"fmt"
	"os"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// DumpFlags are dump's command-line values as cobra parsed them.
type DumpFlags struct {
	Browser     string
	ProfilePath string
	Categories  string
	Output      OutputFlags
}

type DumpOptions struct {
	Browsers    []browser.Browser
	ProfilePath string // -p, for the one browser in Browsers; "" when not given
	HomeDir     string
	Categories  []category.Category
	Output      OutputOptions
	// KeychainPassword runs when the macOS login keychain first opens; "" leaves it locked. Other systems ignore it.
	KeychainPassword func(ctx context.Context) (string, error)
	FirefoxPassword  func(ctx context.Context) (string, error) // nil is the empty password; Dump reads it at most once
}

// ValidateDump checks every flag before any browser or secret is touched, so a bad flag fails fast.
func ValidateDump(flags DumpFlags, keychainPassword, firefoxPassword func(context.Context) (string, error)) (DumpOptions, error) {
	browsers, err := parseBrowsers(flags.Browser)
	if err != nil {
		return DumpOptions{}, err
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return DumpOptions{}, err
	}
	output, err := parseOutput(flags.Output)
	if err != nil {
		return DumpOptions{}, err
	}
	profilePath, err := checkProfilePath(browsers, flags.ProfilePath)
	if err != nil {
		return DumpOptions{}, err
	}
	home, err := homeDir()
	if err != nil {
		return DumpOptions{}, err
	}
	return DumpOptions{
		Browsers:         browsers,
		ProfilePath:      profilePath,
		HomeDir:          home,
		Categories:       categories,
		Output:           output,
		KeychainPassword: keychainPassword,
		FirefoxPassword:  firefoxPassword,
	}, nil
}

// checkProfilePath keeps -p only for one browser; with -b all it is ignored, as it always was, but now said.
func checkProfilePath(browsers []browser.Browser, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if len(browsers) != 1 {
		log.Warnf("-p needs a specific -b; ignoring %s", path)
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("profile path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("profile path %s: not a directory", path)
	}
	return path, nil
}

// Dump exports what it finds on this machine, retrieving each dir's master keys here. A Discovery without dirs means
// nothing ran and nothing was created. An error means the run never started; one that ended early sets Summary.Err.
func Dump(ctx context.Context, options DumpOptions) (Discovery, Summary, error) {
	discovery := discover(ctx, options.Browsers, browser.DiscoverOptions{HomeDir: options.HomeDir, ProfilePath: options.ProfilePath})
	if len(discovery.Dirs) == 0 {
		return discovery, Summary{}, nil
	}
	keychain := newLoginKeychain(loginKeychainPath(options.HomeDir), options.KeychainPassword)
	fromMachine := runner{
		extractors: extractorsFor, retrieveKeys: keysFromMachine(keychain), loginKeychain: keychain,
		firefoxPassword: readOnce(options.FirefoxPassword),
	}
	summary, err := execute(ctx, discovery, options.Categories, options.Output, fromMachine)
	if err != nil {
		return Discovery{}, Summary{}, err
	}
	return discovery, summary, nil
}
