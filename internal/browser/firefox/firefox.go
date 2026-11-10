// Package firefox is the Firefox base: profile discovery, file parsing, and each profile's own key4.db.
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
)

var (
	loginsJSON     = browser.Source{Path: "logins.json"}
	keyStore       = browser.Source{Path: "key4.db", Optional: true}
	placesDB       = browser.Source{Path: "places.sqlite"}
	cookiesDB      = browser.Source{Path: "cookies.sqlite"}
	extensionsJSON = browser.Source{Path: "extensions.json"}
	webappsStore   = browser.Source{Path: "webappsstore.sqlite"}
)

// defaultExtractors has no credit card or session storage extractor, because Firefox keeps none that HBD reads.
var defaultExtractors = browser.Extractors{
	category.Password:     newDecryptingExtractor(extractPasswords, loginsJSON),
	category.Cookie:       newPlainExtractor(extractCookies, cookiesDB),
	category.Bookmark:     newPlainExtractor(extractBookmarks, placesDB),
	category.History:      newPlainExtractor(extractHistories, placesDB),
	category.Download:     newPlainExtractor(extractDownloads, placesDB),
	category.Extension:    newPlainExtractor(extractExtensions, extensionsJSON),
	category.LocalStorage: newPlainExtractor(extractLocalStorage, webappsStore),
}

// Discover takes -p as one profile: its parent is the Profiles directory, and only that profile is selected.
func Discover(firefoxBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	if options.ProfilePath != "" {
		return profileAt(firefoxBrowser, options.ProfilePath), nil
	}
	var errs []error
	var dirs []browser.UserDataDir
	for i, path := range userDataDirPaths(firefoxBrowser, options) {
		dir := browser.UserDataDir{ID: firefoxBrowser.UserDataDirID(i), Browser: firefoxBrowser, Path: path}
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

func profileAt(firefoxBrowser browser.Browser, path string) []browser.UserDataDir {
	clean := filepath.Clean(path)
	if !hasDataFile(clean) {
		return nil
	}
	name := filepath.Base(clean)
	return []browser.UserDataDir{{
		ID:       firefoxBrowser.UserDataDirID(0),
		Browser:  firefoxBrowser,
		Path:     filepath.Dir(clean),
		Profiles: []browser.Profile{{Name: name, RelativePath: name}},
	}}
}

func userDataDirPaths(firefoxBrowser browser.Browser, options browser.DiscoverOptions) []string {
	paths := make([]string, 0, len(firefoxBrowser.Platform().UserDataDirs))
	for _, relativePath := range firefoxBrowser.Platform().UserDataDirs {
		paths = append(paths, filepath.Join(options.HomeDir, filepath.FromSlash(relativePath)))
	}
	return paths
}

// discoverProfiles counts any subdirectory with a file an extractor reads, as names are random; key4.db does not.
func discoverProfiles(dir browser.UserDataDir) ([]browser.Profile, error) {
	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		return nil, err
	}
	var profiles []browser.Profile
	for _, entry := range entries {
		if entry.IsDir() && hasDataFile(filepath.Join(dir.Path, entry.Name())) {
			profiles = append(profiles, browser.Profile{Name: entry.Name(), RelativePath: entry.Name()})
		}
	}
	return profiles, nil
}

func hasDataFile(path string) bool {
	for _, extractor := range defaultExtractors {
		for _, source := range extractor.Sources {
			if source.Optional {
				continue
			}
			for _, relativePath := range source.Paths() {
				if _, err := os.Stat(filepath.Join(path, filepath.FromSlash(relativePath))); err == nil {
					return true
				}
			}
		}
	}
	return false
}

func Extractors(_ browser.UserDataDir, _ browser.Profile) browser.Extractors {
	extractors := make(browser.Extractors, len(defaultExtractors))
	for dataCategory, extractor := range defaultExtractors {
		extractors[dataCategory] = extractor
	}
	return extractors
}

func newPlainExtractor(parse func(string, browser.Collector) error, source browser.Source) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{source},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			path, err := profileSnapshot.Path(source)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}

// newDecryptingExtractor also captures key4.db, the profile's own key store.
func newDecryptingExtractor(parse func(func() ([]byte, error), string, browser.Collector) error, source browser.Source) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{source, keyStore},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			path, err := profileSnapshot.Path(source)
			if err != nil {
				return err
			}
			masterKey := func() ([]byte, error) { return profileKey(profileSnapshot, path) }
			return parse(masterKey, path, out)
		},
	}
}

func profileKey(profileSnapshot browser.ProfileSnapshot, loginsPath string) ([]byte, error) {
	key4Path, err := profileSnapshot.Path(keyStore)
	if err != nil {
		return nil, err
	}
	return retrieveMasterKey(key4Path, loginsPath)
}
