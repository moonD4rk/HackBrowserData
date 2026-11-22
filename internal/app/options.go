package app

import (
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

// OutputFlags are the output flags dump and restore share, as cobra parsed them.
type OutputFlags struct {
	Format          string
	Dir             string
	Zip             bool
	MaxStorageValue int
}

// OutputOptions is where and how a dump or a restore writes records.
type OutputOptions struct {
	Dir             string
	Format          export.Format
	Zip             bool
	MaxStorageValue int // localStorage/sessionStorage value characters kept before a size marker replaces it; <= 0 keeps all
}

// DefaultMaxStorageValue bounds a stored value's characters unless --max-storage-value overrides it; 0 disables the bound.
const DefaultMaxStorageValue = 2048

func parseOutput(flags OutputFlags) (OutputOptions, error) {
	format, err := export.ParseFormat(flags.Format)
	if err != nil {
		return OutputOptions{}, err
	}
	if err := checkOutputDir(flags.Dir); err != nil {
		return OutputOptions{}, err
	}
	if flags.MaxStorageValue < 0 {
		return OutputOptions{}, fmt.Errorf("--max-storage-value must be >= 0 (0 means unlimited), got %d", flags.MaxStorageValue)
	}
	return OutputOptions{Dir: flags.Dir, Format: format, Zip: flags.Zip, MaxStorageValue: flags.MaxStorageValue}, nil
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return home, nil
}

const allBrowsers = "all"

// parseBrowsers accepts all or one of this OS's browser names; a name from another OS's table has nothing to find here.
func parseBrowsers(value string) ([]browser.Browser, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" || name == allBrowsers {
		return browser.Supported(), nil
	}
	for _, supportedBrowser := range browser.Supported() {
		if supportedBrowser.Name == name {
			return []browser.Browser{supportedBrowser}, nil
		}
	}
	return nil, fmt.Errorf("unknown browser %q, available: %s|%s", value, allBrowsers, strings.Join(browser.Names(), "|"))
}

// parseCategories keeps a repeated category once, or its records would be exported twice.
func parseCategories(value string) ([]category.Category, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "all") {
		return category.All(), nil
	}
	var categories []category.Category
	added := make(map[category.Category]bool)
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		dataCategory, err := category.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("%w, available: all|%s", err, strings.Join(category.Names(), ","))
		}
		if !added[dataCategory] {
			added[dataCategory] = true
			categories = append(categories, dataCategory)
		}
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("no categories specified")
	}
	return categories, nil
}

// checkOutputDir creates nothing: -d, or its nearest existing ancestor, must be a directory we can create in.
func checkOutputDir(outputDir string) error {
	path := filepath.Clean(outputDir)
	for {
		info, err := os.Stat(path)
		switch {
		case err == nil && info.IsDir():
			if err := canCreateIn(path); err != nil {
				return fmt.Errorf("output dir %s: %w", outputDir, err)
			}
			return nil
		case err == nil:
			return fmt.Errorf("output dir %s: %s is not a directory", outputDir, path)
		case !errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("output dir %s: %w", outputDir, err)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return fmt.Errorf("output dir %s: no existing parent directory", outputDir)
		}
		path = parent
	}
}

// checkOutputFile creates nothing: -o is written whole and renamed into place, so its directory must already exist.
func checkOutputFile(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("output file %s: directory %s does not exist", path, parent)
	case err != nil:
		return fmt.Errorf("output file %s: %w", path, err)
	case !info.IsDir():
		return fmt.Errorf("output file %s: %s is not a directory", path, parent)
	}
	if err := canCreateIn(parent); err != nil {
		return fmt.Errorf("output file %s: %w", path, err)
	}
	if existing, err := os.Stat(path); err == nil && existing.IsDir() {
		return fmt.Errorf("output file %s: is a directory", path)
	}
	return nil
}
