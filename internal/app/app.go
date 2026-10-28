// Package app runs the pipeline: Validate turns flags into a Request, Discover finds the user data dirs, NewPlan crosses
// them with the requested categories, Execute captures, extracts, and exports one user data dir at a time, and Report
// prints what every extraction got.
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

// Flags are a run's command-line values as cobra parsed them.
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

// Request is a validated run.
type Request struct {
	Browser     string // "all" or one Def.ID
	Categories  []category.Category
	Format      export.Format
	Dir         string
	Zip         bool
	ProfilePath string
	Home        string

	// KeychainPassword yields the macOS login password when the login keychain is first opened; "" leaves it locked.
	KeychainPassword func(ctx context.Context) (string, error)
	// Keys are a dump's keys by user data dir; nil retrieves every dir's keys from this machine.
	Keys map[browser.DirID]masterkey.Set
}

// Validate checks every flag before any browser is touched or any secret is read, and resolves the home directory, so
// nothing later reads the environment.
func Validate(f Flags) (Request, error) {
	return validate(f, os.Stdin)
}

func validate(f Flags, stdin *os.File) (Request, error) {
	id, err := parseBrowser(f.Browser)
	if err != nil {
		return Request{}, err
	}
	categories, err := parseCategories(f.Categories)
	if err != nil {
		return Request{}, err
	}
	format, err := export.ParseFormat(f.Format)
	if err != nil {
		return Request{}, err
	}
	if err := checkOutputDir(f.Dir); err != nil {
		return Request{}, err
	}
	profilePath, err := checkProfilePath(id, f.ProfilePath)
	if err != nil {
		return Request{}, err
	}
	password, err := keychainPassword(f, stdin)
	if err != nil {
		return Request{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Request{}, fmt.Errorf("home directory: %w", err)
	}
	return Request{
		Browser:          id,
		Categories:       categories,
		Format:           format,
		Dir:              f.Dir,
		Zip:              f.Zip,
		ProfilePath:      profilePath,
		Home:             home,
		KeychainPassword: password,
	}, nil
}

// parseBrowser accepts all or one of this OS's browser IDs; an ID from another OS's table has nothing to find here.
func parseBrowser(s string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(s))
	if id == "" || id == "all" {
		return "all", nil
	}
	ids := []string{"all"}
	for _, def := range browser.Supported() {
		if def.ID == id {
			return id, nil
		}
		ids = append(ids, def.ID)
	}
	return "", fmt.Errorf("unknown browser %q, available: %s", s, strings.Join(ids, "|"))
}

// checkOutputDir creates nothing: -d must already be a directory, or its nearest existing ancestor must be one that a
// directory can be created in.
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
func checkProfilePath(id, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if id == "all" {
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

// parseCategories reads a comma-separated -c value; a repeated category is kept once, or its records would be
// exported twice.
func parseCategories(s string) ([]category.Category, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "all") {
		return category.All(), nil
	}
	var categories []category.Category
	seen := make(map[category.Category]bool)
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		c, err := category.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("%w, available: all|%s", err, categoryNames())
		}
		if !seen[c] {
			seen[c] = true
			categories = append(categories, c)
		}
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("no categories specified")
	}
	return categories, nil
}

func categoryNames() string {
	all := category.All()
	names := make([]string, len(all))
	for i, c := range all {
		names[i] = c.String()
	}
	return strings.Join(names, ",")
}
