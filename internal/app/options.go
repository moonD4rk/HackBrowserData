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
)

// Options are a command's validated flags, grouped by the stage that reads them; a command leaves the rest empty.
type Options struct {
	Target     Target
	Categories []category.Category
	Output     Output
	Keychain   KeychainConfig
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
