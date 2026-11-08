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

// Extractors builds a named profile's specs from Profile.RelativePath; its files sit under two dirs named for its UUID.
// The keychain, Bookmarks.plist, and the extension plists carry no profile tag, so only the default profile reads them.
func Extractors(dir browser.UserDataDir, profile browser.Profile, loginKeychain LoginKeychain) browser.Extractors {
	library := func(relativePath string) string { return fromLibrary(profile, relativePath) }
	uuid, named := namedProfileUUID(profile)
	if !named {
		return browser.Extractors{
			category.Password: newPasswordExtractor(loginKeychain),
			category.History:  newPlainExtractor(newFileSpec("History.db"), extractHistories),
			category.Cookie: newPlainExtractor(newFileSpec(
				library(containerDir+"/Cookies/Cookies.binarycookies"),
				library("Cookies/Cookies.binarycookies"),
			), extractCookies),
			category.Bookmark:     newPlainExtractor(newFileSpec("Bookmarks.plist"), extractBookmarks),
			category.Download:     newPlainExtractor(newFileSpec("Downloads.plist"), downloadsOf(defaultProfileSentinel)),
			category.LocalStorage: newPlainExtractor(newDirectorySpec(library(containerDir+"/WebKit/WebsiteData/Default")), extractLocalStorage),
			category.Extension:    newExtensionExtractor(library),
		}
	}
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(uuid)
	return browser.Extractors{
		category.History:      newPlainExtractor(newFileSpec("History.db"), extractHistories),
		category.Cookie:       newPlainExtractor(newFileSpec(library(store+"/Cookies/Cookies.binarycookies")), extractCookies),
		category.Download:     newPlainExtractor(newFileSpec(library(defaultProfileDir(dir)+"/Downloads.plist")), downloadsOf(uuid)),
		category.LocalStorage: newPlainExtractor(newDirectorySpec(library(store+"/Origins")), extractLocalStorage),
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

// fromLibrary turns a path under ~/Library into one relative to profile.RelativePath, which file specs are joined with.
func fromLibrary(profile browser.Profile, relativePath string) string {
	if profile.RelativePath == "" {
		return relativePath
	}
	return strings.Repeat("../", strings.Count(profile.RelativePath, "/")+1) + relativePath
}

func newFileSpec(paths ...string) browser.FileSpec { return browser.FileSpec{Paths: paths} }
func newDirectorySpec(relativePath string) browser.FileSpec {
	return browser.FileSpec{Paths: []string{relativePath}, IsDir: true}
}

func newPlainExtractor(fileSpec browser.FileSpec, parse func(string, browser.Collector) error) browser.Extractor {
	return browser.Extractor{
		FileSpecs: []browser.FileSpec{fileSpec},
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			captured, err := profileSnapshot.Path(fileSpec)
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

func newExtensionExtractor(library func(string) string) browser.Extractor {
	fileSpecs := []browser.FileSpec{
		{Paths: []string{library(containerDir + "/Safari/AppExtensions/Extensions.plist")}, Optional: true},
		{Paths: []string{library(containerDir + "/Safari/WebExtensions/Extensions.plist")}, Optional: true},
	}
	return browser.Extractor{
		FileSpecs: fileSpecs,
		Extract: func(_ context.Context, profileSnapshot browser.ProfileSnapshot, out browser.Collector) error {
			var paths []string
			for _, fileSpec := range fileSpecs {
				captured, err := profileSnapshot.Path(fileSpec)
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
