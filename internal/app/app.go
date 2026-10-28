// Package app runs the pipeline: Validate turns flags into a Request, Discover finds the user data dirs, NewPlan crosses
// them with the requested categories, Execute captures, extracts, and exports one user data dir at a time, and Report
// prints what every extraction got.
package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// Flags are a run's command-line values as cobra parsed them.
type Flags struct {
	Browser          string
	Categories       string
	Format           string
	Dir              string
	Zip              bool
	ProfilePath      string
	KeychainPassword string
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

// Validate checks the flags and resolves the home directory, so nothing later reads the environment.
func Validate(f Flags) (Request, error) {
	categories, err := parseCategories(f.Categories)
	if err != nil {
		return Request{}, err
	}
	format, err := export.ParseFormat(f.Format)
	if err != nil {
		return Request{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Request{}, fmt.Errorf("home directory: %w", err)
	}
	name := strings.ToLower(strings.TrimSpace(f.Browser))
	if name == "" {
		name = "all"
	}
	password := f.KeychainPassword
	return Request{
		Browser:          name,
		Categories:       categories,
		Format:           format,
		Dir:              f.Dir,
		Zip:              f.Zip,
		ProfilePath:      f.ProfilePath,
		Home:             home,
		KeychainPassword: func(context.Context) (string, error) { return password, nil },
	}, nil
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
