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
	loginsJSON = browser.FileSpec{Paths: []string{"logins.json"}}
	keyStore   = browser.FileSpec{Paths: []string{"key4.db"}, Optional: true}
	placesDB   = browser.FileSpec{Paths: []string{"places.sqlite"}}
)

// defaultExtractors has no credit card or session storage extractor, because Firefox keeps none that HBD reads.
var defaultExtractors = browser.Extractors{
	category.Password:     newDecryptingExtractor(extractPasswords, loginsJSON),
	category.Cookie:       newPlainExtractor(extractCookies, newFileSpec("cookies.sqlite")),
	category.Bookmark:     newPlainExtractor(extractBookmarks, placesDB),
	category.History:      newPlainExtractor(extractHistories, placesDB),
	category.Download:     newPlainExtractor(extractDownloads, placesDB),
	category.Extension:    newPlainExtractor(extractExtensions, newFileSpec("extensions.json")),
	category.LocalStorage: newPlainExtractor(extractLocalStorage, newFileSpec("webappsstore.sqlite")),
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

func hasDataFile(dir string) bool {
	for _, extractor := range defaultExtractors {
		for _, fileSpec := range extractor.FileSpecs {
			if fileSpec.Optional {
				continue
			}
			for _, relativePath := range fileSpec.Paths {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(relativePath))); err == nil {
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

func newFileSpec(path string) browser.FileSpec { return browser.FileSpec{Paths: []string{path}} }

func newPlainExtractor(parse func(string, browser.Collector) error, fileSpec browser.FileSpec) browser.Extractor {
	return browser.Extractor{
		FileSpecs: []browser.FileSpec{fileSpec},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			path, err := profileSnapshot.Path(fileSpec)
			if err != nil {
				return err
			}
			return parse(path, out)
		},
	}
}

// newDecryptingExtractor also captures key4.db, the profile's own key store.
func newDecryptingExtractor(parse func(func() []byte, string, browser.Collector) error, fileSpec browser.FileSpec) browser.Extractor {
	return browser.Extractor{
		FileSpecs: []browser.FileSpec{fileSpec, keyStore},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			path, err := profileSnapshot.Path(fileSpec)
			if err != nil {
				return err
			}
			return parse(masterKeyOf(profileSnapshot, path, out), path, out)
		},
	}
}

// masterKeyOf derives the key only once a login needs it; without one the logins export with empty secrets.
func masterKeyOf(profileSnapshot browser.ProfileSnapshot, loginsPath string, out browser.Collector) func() []byte {
	var (
		key     []byte
		derived bool
	)
	return func() []byte {
		if !derived {
			derived = true
			retrieved, err := profileKey(profileSnapshot, loginsPath)
			if err != nil {
				out.Fail(fmt.Errorf("master key: %w", err))
			}
			key = retrieved
		}
		return key
	}
}

func profileKey(profileSnapshot browser.ProfileSnapshot, loginsPath string) ([]byte, error) {
	key4Path, err := profileSnapshot.Path(keyStore)
	if err != nil {
		return nil, err
	}
	return retrieveMasterKey(key4Path, loginsPath)
}
