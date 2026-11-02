// Package app runs the pipeline: Validate, Discover, NewPlan, Execute, and Report.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

type Flags struct {
	Browser               string
	Categories            string
	Format                string
	Dir                   string
	Zip                   bool
	ProfilePath           string
	KeychainPassword      string
	KeychainPasswordStdin bool
}

type Options struct {
	Browser     string // "all" or one Browser.Name
	Categories  []category.Category
	Format      export.Format
	Dir         string
	Zip         bool
	ProfilePath string
	Home        string

	// KeychainPassword runs when the keychain first opens; "" leaves it locked, and nil, as list has it, silently.
	KeychainPassword func(ctx context.Context) (string, error)
	// Keys are a dump's keys by user data dir; nil retrieves every dir's keys from this machine.
	Keys map[browser.UserDataDirID]masterkey.Set
}

// Validate checks every flag before any browser or secret is touched, so a bad flag fails fast.
func Validate(flags Flags) (Options, error) {
	return validate(flags, os.Stdin)
}

func validate(flags Flags, stdin *os.File) (Options, error) {
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
	if err := checkOutputDir(flags.Dir); err != nil {
		return Options{}, err
	}
	profilePath, err := checkProfilePath(browserName, flags.ProfilePath)
	if err != nil {
		return Options{}, err
	}
	password, err := keychainPassword(flags, stdin)
	if err != nil {
		return Options{}, err
	}
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{
		Browser:          browserName,
		Categories:       categories,
		Format:           format,
		Dir:              flags.Dir,
		Zip:              flags.Zip,
		ProfilePath:      profilePath,
		Home:             home,
		KeychainPassword: password,
	}, nil
}

func ValidateKeys(flags Flags) (Options, error) {
	browserName, err := parseBrowser(flags.Browser)
	if err != nil {
		return Options{}, err
	}
	password, err := keychainPassword(flags, os.Stdin)
	if err != nil {
		return Options{}, err
	}
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{Browser: browserName, Home: home, KeychainPassword: password}, nil
}

// ValidateArchive takes no keychain password: an archive copies files and reads no secret.
func ValidateArchive(flags Flags) (Options, error) {
	browserName, err := parseBrowser(flags.Browser)
	if err != nil {
		return Options{}, err
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return Options{}, err
	}
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{Browser: browserName, Categories: categories, Home: home}, nil
}

// ListOptions is what list reads: every browser and category, with the keychain left locked, so counting never prompts.
func ListOptions() (Options, error) {
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{Browser: allBrowsers, Categories: category.All(), Home: home}, nil
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return home, nil
}

const allBrowsers = "all"

// parseBrowser accepts all or one of this OS's browser names; a name from another OS's table has nothing to find here.
func parseBrowser(value string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" || name == allBrowsers {
		return allBrowsers, nil
	}
	for _, supportedBrowser := range browser.Supported() {
		if supportedBrowser.Name == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown browser %q, available: %s|%s", value, allBrowsers, strings.Join(browser.Names(), "|"))
}

// checkOutputDir creates nothing: -d, or its nearest existing ancestor, must be a directory we can create in.
func checkOutputDir(dir string) error {
	path := filepath.Clean(dir)
	for {
		info, err := os.Stat(path)
		switch {
		case err == nil && info.IsDir():
			if err := canCreateIn(path); err != nil {
				return fmt.Errorf("output dir %s: %w", dir, err)
			}
			return nil
		case err == nil:
			return fmt.Errorf("output dir %s: %s is not a directory", dir, path)
		case !errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("output dir %s: %w", dir, err)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return fmt.Errorf("output dir %s: no existing parent directory", dir)
		}
		path = parent
	}
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

// parseCategories keeps a repeated category once, or its records would be exported twice.
func parseCategories(value string) ([]category.Category, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "all") {
		return category.All(), nil
	}
	var categories []category.Category
	seen := make(map[category.Category]bool)
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		dataCategory, err := category.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("%w, available: all|%s", err, strings.Join(category.Names(), ","))
		}
		if !seen[dataCategory] {
			seen[dataCategory] = true
			categories = append(categories, dataCategory)
		}
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("no categories specified")
	}
	return categories, nil
}
