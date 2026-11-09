// Package safari is the Safari base: default and named profiles under ~/Library, their files, and the login keychain.
package safari

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/moond4rk/keychainbreaker"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

// Paths below are relative to the user data dir, ~/Library.
const (
	containerDir     = "Containers/com.apple.Safari/Data/Library"
	namedProfilesDir = containerDir + "/Safari/Profiles"

	defaultProfileName     = "default"
	discoveryUserDataDirID = "safari"
)

var errNoKeychain = errors.New("no login keychain")

// LoginKeychain is the one method the password extractor reads from the run's login keychain.
type LoginKeychain interface {
	InternetPasswords(ctx context.Context) ([]keychainbreaker.InternetPassword, error)
}

// Discover makes the Safari directory the default profile; -p names another, whose parent replaces ~/Library.
func Discover(safariBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	var errs []error
	var dirs []browser.UserDataDir
	for i, safariPath := range userDataDirPaths(safariBrowser, options) {
		if _, err := os.Stat(safariPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("%s: %w", safariPath, err))
			}
			continue
		}
		library := filepath.Dir(safariPath)
		profiles := append([]browser.Profile{{Name: defaultProfileName, RelativePath: filepath.Base(safariPath)}}, namedProfiles(library)...)
		disambiguateNames(profiles)
		dirs = append(dirs, browser.UserDataDir{ID: safariBrowser.UserDataDirID(i), Browser: safariBrowser, Path: library, Profiles: profiles})
	}
	return dirs, errors.Join(errs...)
}

func userDataDirPaths(safariBrowser browser.Browser, options browser.DiscoverOptions) []string {
	if options.ProfilePath != "" {
		return []string{filepath.Clean(options.ProfilePath)}
	}
	paths := make([]string, 0, len(safariBrowser.Platform().UserDataDirs))
	for _, relativePath := range safariBrowser.Platform().UserDataDirs {
		paths = append(paths, filepath.Join(options.HomeDir, filepath.FromSlash(relativePath)))
	}
	return paths
}

// Extractors builds a named profile's sources from Profile.RelativePath; its files sit under two dirs named for its UUID.
// The keychain, Bookmarks.plist, and the extension plists carry no profile tag, so only the default profile reads them.
func Extractors(dir browser.UserDataDir, profile browser.Profile, loginKeychain LoginKeychain) browser.Extractors {
	if uuid, named := namedProfileUUID(profile); named {
		return namedProfileExtractors(dir, profile, uuid)
	}
	return defaultProfileExtractors(profile, loginKeychain)
}

func defaultProfileExtractors(profile browser.Profile, loginKeychain LoginKeychain) browser.Extractors {
	var (
		history = browser.Source{Path: "History.db"}
		cookies = browser.Source{
			Path:      fromLibrary(profile, containerDir+"/Cookies/Cookies.binarycookies"),
			Fallbacks: []string{fromLibrary(profile, "Cookies/Cookies.binarycookies")},
		}
		bookmarks     = browser.Source{Path: "Bookmarks.plist"}
		downloads     = browser.Source{Path: "Downloads.plist"}
		localStorage  = browser.Source{Path: fromLibrary(profile, containerDir+"/WebKit/WebsiteData/Default"), IsDir: true}
		appExtensions = browser.Source{Path: fromLibrary(profile, containerDir+"/Safari/AppExtensions/Extensions.plist"), Optional: true}
		webExtensions = browser.Source{Path: fromLibrary(profile, containerDir+"/Safari/WebExtensions/Extensions.plist"), Optional: true}
	)
	return browser.Extractors{
		category.Password:     newPasswordExtractor(loginKeychain),
		category.History:      newPlainExtractor(extractHistories, history),
		category.Cookie:       newPlainExtractor(extractCookies, cookies),
		category.Bookmark:     newPlainExtractor(extractBookmarks, bookmarks),
		category.Download:     newPlainExtractor(downloadsOf(defaultProfileSentinel), downloads),
		category.LocalStorage: newPlainExtractor(extractLocalStorage, localStorage),
		category.Extension:    newExtensionExtractor(appExtensions, webExtensions),
	}
}

func namedProfileExtractors(dir browser.UserDataDir, profile browser.Profile, uuid string) browser.Extractors {
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(uuid)
	var (
		history      = browser.Source{Path: "History.db"}
		cookies      = browser.Source{Path: fromLibrary(profile, store+"/Cookies/Cookies.binarycookies")}
		downloads    = browser.Source{Path: fromLibrary(profile, defaultProfileDir(dir)+"/Downloads.plist")}
		localStorage = browser.Source{Path: fromLibrary(profile, store+"/Origins"), IsDir: true}
	)
	return browser.Extractors{
		category.History:      newPlainExtractor(extractHistories, history),
		category.Cookie:       newPlainExtractor(extractCookies, cookies),
		category.Download:     newPlainExtractor(downloadsOf(uuid), downloads),
		category.LocalStorage: newPlainExtractor(extractLocalStorage, localStorage),
	}
}

func namedProfileUUID(profile browser.Profile) (string, bool) {
	if path.Dir(profile.RelativePath) != namedProfilesDir {
		return "", false
	}
	return path.Base(profile.RelativePath), true
}

// defaultProfileDir is where the default profile lives; its Downloads.plist holds every profile's entries.
func defaultProfileDir(dir browser.UserDataDir) string {
	for _, profile := range dir.Profiles {
		if _, named := namedProfileUUID(profile); !named {
			return profile.RelativePath
		}
	}
	return "Safari"
}

// fromLibrary turns a path under ~/Library into one relative to profile.RelativePath, which sources are joined with.
func fromLibrary(profile browser.Profile, relativePath string) string {
	if profile.RelativePath == "" {
		return relativePath
	}
	return strings.Repeat("../", strings.Count(profile.RelativePath, "/")+1) + relativePath
}

func newPlainExtractor(parse func(string, browser.Collector) error, source browser.Source) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{source},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			captured, err := profileSnapshot.Path(source)
			if err != nil {
				return err
			}
			return parse(captured, out)
		},
	}
}

// newPasswordExtractor lists no file, so it always runs; without a usable password the keychain yields metadata only.
func newPasswordExtractor(loginKeychain LoginKeychain) browser.Extractor {
	return browser.Extractor{Extract: func(ctx context.Context, _ browser.ProfileSnapshot, out browser.Collector) error {
		if loginKeychain == nil {
			return errNoKeychain
		}
		return extractPasswords(ctx, loginKeychain, out)
	}}
}

func downloadsOf(profileUUID string) func(string, browser.Collector) error {
	return func(path string, out browser.Collector) error { return extractDownloads(path, profileUUID, out) }
}

// newExtensionExtractor merges every plist, so the extensions come out sorted across both stores.
func newExtensionExtractor(sources ...browser.Source) browser.Extractor {
	return browser.Extractor{
		Sources: sources,
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			var paths []string
			for _, source := range sources {
				captured, err := profileSnapshot.Path(source)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					return err
				}
				paths = append(paths, captured)
			}
			return extractExtensions(paths, out)
		},
	}
}
