// Package firefox is the Firefox base: where Firefox keeps its profiles, how their files are parsed, and how each
// profile's own key4.db opens its logins.
package firefox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var (
	loginsJSON = browser.Source{Paths: []string{"logins.json"}}
	keyStore   = browser.Source{Paths: []string{"key4.db"}, Optional: true}
	placesDB   = browser.Source{Paths: []string{"places.sqlite"}}
)

// Firefox keeps no credit cards or session storage that HBD reads, so those categories have no extractor.
var extractors = browser.Extractors{
	category.Password:     {Sources: []browser.Source{loginsJSON, keyStore}, Extract: extractLogins},
	category.Cookie:       plain(file("cookies.sqlite"), extractCookies),
	category.Bookmark:     plain(placesDB, extractBookmarks),
	category.History:      plain(placesDB, extractHistories),
	category.Download:     plain(placesDB, extractDownloads),
	category.Extension:    plain(file("extensions.json"), extractExtensions),
	category.LocalStorage: plain(file("webappsstore.sqlite"), extractLocalStorage),
}

// Discover finds def's Profiles directory and the profiles in it. -p names one profile: its parent is the Profiles
// directory, and only that profile is selected.
func Discover(def browser.Def, opts browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	if opts.ProfilePath != "" {
		return profileAt(def, opts.ProfilePath), nil
	}
	var errs []error
	var dirs []browser.UserDataDir
	for i, path := range userDataDirPaths(def, opts) {
		dir := browser.UserDataDir{ID: dirID(def, i), Def: def, Path: path}
		profiles, err := discoverProfiles(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		dir.Profiles = profiles
		dirs = append(dirs, dir)
	}
	return dirs, errors.Join(errs...)
}

func profileAt(def browser.Def, path string) []browser.UserDataDir {
	clean := filepath.Clean(path)
	if !hasDataFile(clean) {
		return nil
	}
	name := filepath.Base(clean)
	return []browser.UserDataDir{{
		ID:       dirID(def, 0),
		Def:      def,
		Path:     filepath.Dir(clean),
		Profiles: []browser.Profile{{Name: name, Dir: name}},
	}}
}

func userDataDirPaths(def browser.Def, opts browser.DiscoverOptions) []string {
	paths := make([]string, 0, len(def.Platform().UserDataDirs))
	for _, rel := range def.Platform().UserDataDirs {
		paths = append(paths, filepath.Join(opts.Home, filepath.FromSlash(rel)))
	}
	return paths
}

func dirID(def browser.Def, i int) browser.DirID {
	if i == 0 {
		return browser.DirID(def.ID)
	}
	return browser.DirID(fmt.Sprintf("%s-%d", def.ID, i+1))
}

// discoverProfiles takes every subdirectory holding a file some category reads, since profile directories have random
// names; key4.db alone does not make one.
func discoverProfiles(dir browser.UserDataDir) ([]browser.Profile, error) {
	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		return nil, err
	}
	var profiles []browser.Profile
	for _, e := range entries {
		if e.IsDir() && hasDataFile(filepath.Join(dir.Path, e.Name())) {
			profiles = append(profiles, browser.Profile{Name: e.Name(), Dir: e.Name()})
		}
	}
	return profiles, nil
}

func hasDataFile(dir string) bool {
	for _, e := range extractors {
		for _, src := range e.Sources {
			if src.Optional {
				continue
			}
			for _, rel := range src.Paths {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
					return true
				}
			}
		}
	}
	return false
}

// Extractors maps every category a Firefox profile supports to its extractor.
func Extractors(_ browser.UserDataDir, _ browser.Profile) browser.Extractors {
	ex := make(browser.Extractors, len(extractors))
	for c, e := range extractors {
		ex[c] = e
	}
	return ex
}

func file(path string) browser.Source { return browser.Source{Paths: []string{path}} }

func plain(src browser.Source, parse func(string, browser.Collector) error) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{src},
		Extract: func(_ context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
			path, err := ps.Path(src)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}

// extractLogins derives the profile's key only once a login needs it. Without a key the logins are still exported,
// their username and password empty, and the reason is the extraction's first problem.
func extractLogins(_ context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
	path, err := ps.Path(loginsJSON)
	if err != nil {
		return err
	}
	var (
		key     []byte
		derived bool
	)
	masterKey := func() []byte {
		if !derived {
			derived = true
			k, keyErr := profileKey(ps, path)
			if keyErr != nil {
				out.Fail(fmt.Errorf("master key: %w", keyErr))
			}
			key = k
		}
		return key
	}
	return extractPasswords(masterKey, path, out)
}

func profileKey(ps browser.ProfileSnapshot, loginsPath string) ([]byte, error) {
	key4Path, err := ps.Path(keyStore)
	if err != nil {
		return nil, err
	}
	return retrieveMasterKey(key4Path, loginsPath)
}

// emit hands rows to out by pointer, because out stamps each record in place.
func emit[T any, P interface {
	*T
	record.Record
}](out browser.Collector, rows []T) {
	for i := range rows {
		out.Add(P(&rows[i]))
	}
}
