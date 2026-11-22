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
	loginsJSON     = browser.Source{RelativePath: "logins.json"}
	keyStore       = browser.Source{RelativePath: "key4.db", Optional: true}
	placesDB       = browser.Source{RelativePath: "places.sqlite"}
	cookiesDB      = browser.Source{RelativePath: "cookies.sqlite"}
	extensionsJSON = browser.Source{RelativePath: "extensions.json"}
	webappsStore   = browser.Source{RelativePath: "webappsstore.sqlite"}
)

// extractor is how one category is read: the files it needs, and how it parses them with the user's primary password.
type extractor struct {
	sources []browser.Source
	extract func(captured browser.ProfileSnapshot, primaryPassword string, out browser.Collector) error
}

// extractors has no credit card or session storage extractor, because Firefox keeps none that HBD reads.
var extractors = map[category.Category]extractor{
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
		if dir, ok := dirFromProfilePath(firefoxBrowser, options.ProfilePath); ok {
			return []browser.UserDataDir{dir}, nil
		}
		return nil, nil
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

func dirFromProfilePath(firefoxBrowser browser.Browser, path string) (browser.UserDataDir, bool) {
	clean := filepath.Clean(path)
	if !hasDataFile(clean) {
		return browser.UserDataDir{}, false
	}
	name := filepath.Base(clean)
	return browser.UserDataDir{
		ID:       firefoxBrowser.UserDataDirID(0),
		Browser:  firefoxBrowser,
		Path:     filepath.Dir(clean),
		Profiles: []browser.Profile{{Name: name, RelativePath: name}},
	}, true
}

func userDataDirPaths(firefoxBrowser browser.Browser, options browser.DiscoverOptions) []string {
	paths := make([]string, 0, len(firefoxBrowser.Platform().UserDataDirPaths))
	for _, relativePath := range firefoxBrowser.Platform().UserDataDirPaths {
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
	for _, extractor := range extractors {
		for _, source := range extractor.sources {
			if source.Optional {
				continue
			}
			for _, relativePath := range source.RelativePaths() {
				if _, err := os.Stat(filepath.Join(path, filepath.FromSlash(relativePath))); err == nil {
					return true
				}
			}
		}
	}
	return false
}

// Sources lists the files of each category Firefox supports; every profile reads the same ones.
func Sources(_ browser.UserDataDir, _ browser.Profile) map[category.Category][]browser.Source {
	sources := make(map[category.Category][]browser.Source, len(extractors))
	for dataCategory, extractor := range extractors {
		sources[dataCategory] = extractor.sources
	}
	return sources
}

// Extract opens a protected profile's key store with primaryPassword, "" when the user gave none.
func Extract(
	_ context.Context, captured browser.ProfileSnapshot, primaryPassword string, dataCategory category.Category, out browser.Collector,
) error {
	extractor, ok := extractors[dataCategory]
	if !ok {
		return fmt.Errorf("%s has no %s extractor", captured.UserDataDir.Browser.Name, dataCategory)
	}
	return extractor.extract(captured, primaryPassword, out)
}

func newPlainExtractor(extractFile func(string, browser.Collector) error, source browser.Source) extractor {
	return extractor{
		sources: []browser.Source{source},
		extract: func(captured browser.ProfileSnapshot, _ string, out browser.Collector) error {
			path, err := captured.Path(source)
			if err != nil {
				return err
			}
			return extractFile(path, out)
		},
	}
}

// newDecryptingExtractor also captures key4.db, the profile's own key store.
func newDecryptingExtractor(extractFile func(func() ([]byte, error), string, browser.Collector) error, source browser.Source) extractor {
	return extractor{
		sources: []browser.Source{source, keyStore},
		extract: func(captured browser.ProfileSnapshot, primaryPassword string, out browser.Collector) error {
			path, err := captured.Path(source)
			if err != nil {
				return err
			}
			masterKey := func() ([]byte, error) { return profileKey(captured, path, primaryPassword) }
			return extractFile(masterKey, path, out)
		},
	}
}

func profileKey(captured browser.ProfileSnapshot, loginsPath, primaryPassword string) ([]byte, error) {
	key4Path, err := captured.Path(keyStore)
	if err != nil {
		return nil, err
	}
	return retrieveMasterKey(key4Path, loginsPath, []byte(primaryPassword))
}
