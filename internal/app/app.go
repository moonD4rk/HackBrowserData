// Package app runs the pipeline: the Validate functions, Discover, NewPlan, Execute, and Report.
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
)

// DumpFlags are dump's command-line values as cobra parsed them.
type DumpFlags struct {
	Browser     string
	Categories  string
	Format      string
	OutputDir   string
	Zip         bool
	ProfilePath string
	Keychain    KeychainFlags
}

// DumpKeysFlags are dumpkeys' command-line values as cobra parsed them.
type DumpKeysFlags struct {
	Browser  string
	Keychain KeychainFlags
}

// ArchiveFlags are archive's command-line values as cobra parsed them.
type ArchiveFlags struct {
	Browser    string
	Categories string
}

// KeychainFlags say where the macOS login keychain password comes from; both empty means a prompt, or none.
type KeychainFlags struct {
	Password      string
	PasswordStdin bool
}

// Target is what Discover looks for.
type Target struct {
	BrowserName string // "all" or one Browser.Name
	ProfilePath string // a custom profile dir, used only with one browser
	HomeDir     string
}

// Output is where and how Execute writes records.
type Output struct {
	Dir    string
	Format export.Format
	Zip    bool
}

// KeychainConfig locates the macOS login keychain and says how to get its password; other systems ignore it.
type KeychainConfig struct {
	HomeDir string
	// Password runs when the keychain first opens; "" leaves it locked, and nil, as list has it, silently.
	Password func(ctx context.Context) (string, error)
}

// Options are a command's validated flags, grouped by the stage that reads them; a command leaves the rest empty.
type Options struct {
	Target     Target
	Categories []category.Category
	Output     Output
	Keychain   KeychainConfig
}

// ValidateDump checks every flag before any browser or secret is touched, so a bad flag fails fast.
func ValidateDump(flags DumpFlags) (Options, error) {
	return validateDump(flags, os.Stdin)
}

func validateDump(flags DumpFlags, stdin *os.File) (Options, error) {
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
	password, err := keychainPasswordFunc(flags.Keychain, stdin)
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
		Keychain:   KeychainConfig{HomeDir: home, Password: password},
	}, nil
}

func ValidateDumpKeys(flags DumpKeysFlags) (Options, error) {
	browserName, err := parseBrowser(flags.Browser)
	if err != nil {
		return Options{}, err
	}
	password, err := keychainPasswordFunc(flags.Keychain, os.Stdin)
	if err != nil {
		return Options{}, err
	}
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{
		Target:   Target{BrowserName: browserName, HomeDir: home},
		Keychain: KeychainConfig{HomeDir: home, Password: password},
	}, nil
}

// ValidateArchive takes no keychain password: an archive copies files and reads no secret.
func ValidateArchive(flags ArchiveFlags) (Options, error) {
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
	return Options{Target: Target{BrowserName: browserName, HomeDir: home}, Categories: categories}, nil
}

// ListOptions is what list reads: every browser and category, with the keychain left locked, so counting never prompts.
func ListOptions() (Options, error) {
	home, err := homeDir()
	if err != nil {
		return Options{}, err
	}
	return Options{
		Target:     Target{BrowserName: allBrowsers, HomeDir: home},
		Categories: category.All(),
		Keychain:   KeychainConfig{HomeDir: home},
	}, nil
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
