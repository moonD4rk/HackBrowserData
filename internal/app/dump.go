package app

import (
	"context"
	"fmt"
	"os"

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

// ValidateDump checks every flag before any browser or secret is touched, so a bad flag fails fast.
func ValidateDump(flags DumpFlags, keychainPassword func(context.Context) (string, error)) (Options, error) {
	browserName, err := parseBrowser(flags.Browser)
	if err != nil {
		return Options{}, err
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return Options{}, err
	}
	format, err := export.ParseFormat(flags.Format)
	if err != nil {
		return Options{}, err
	}
	if err := checkOutputDir(flags.OutputDir); err != nil {
		return Options{}, err
	}
	profilePath, err := checkProfilePath(browserName, flags.ProfilePath)
	if err != nil {
		return Options{}, err
	}
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{
		Target:     Target{BrowserName: browserName, ProfilePath: profilePath, HomeDir: home},
		Categories: categories,
		Output:     Output{Dir: flags.OutputDir, Format: format, Zip: flags.Zip},
		Keychain:   KeychainConfig{HomeDir: home, Password: keychainPassword},
	}, nil
}

// checkProfilePath keeps -p only when one browser is named; with -b all it is ignored, as it always was, but now said.
func checkProfilePath(browserName, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if browserName == allBrowsers {
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
