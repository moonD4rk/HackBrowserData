package app

import (
	"context"
	"fmt"
	"os"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// DumpFlags are dump's command-line values as cobra parsed them.
type DumpFlags struct {
	Browser     string
	Categories  string
	Format      string
	OutputDir   string
	Zip         bool
	ProfilePath string
}

type DumpOptions struct {
	Target     Target
	Categories []category.Category
	Output     Output
	Keychain   KeychainConfig
}

// ValidateDump checks every flag before any browser or secret is touched, so a bad flag fails fast.
func ValidateDump(flags DumpFlags, keychainPassword func(context.Context) (string, error)) (DumpOptions, error) {
	browsers, err := parseBrowsers(flags.Browser)
	if err != nil {
		return DumpOptions{}, err
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return DumpOptions{}, err
	}
	format, err := export.ParseFormat(flags.Format)
	if err != nil {
		return DumpOptions{}, err
	}
	if err := checkOutputDir(flags.OutputDir); err != nil {
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
		Target:     Target{Browsers: browsers, ProfilePath: profilePath, HomeDir: home},
		Categories: categories,
		Output:     Output{Dir: flags.OutputDir, Format: format, Zip: flags.Zip},
		Keychain:   KeychainConfig{Path: loginKeychainPath(home), Password: keychainPassword},
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
