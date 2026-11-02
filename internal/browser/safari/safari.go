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
	"github.com/moond4rk/hackbrowserdata/internal/record"
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

// Discover makes the Safari directory the default profile's Dir; -p names another, whose parent replaces ~/Library.
func Discover(safariBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	var errs []error
	var dirs []browser.UserDataDir
	for i, safariPath := range safariPaths(safariBrowser, options) {
		if _, err := os.Stat(safariPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("%s: %w", safariPath, err))
			}
			continue
		}
		library := filepath.Dir(safariPath)
		profiles := append([]browser.Profile{{Name: defaultProfileName, Dir: filepath.Base(safariPath)}}, namedProfiles(library)...)
		disambiguateNames(profiles)
		dirs = append(dirs, browser.UserDataDir{ID: safariBrowser.UserDataDirID(i), Browser: safariBrowser, Path: library, Profiles: profiles})
	}
	return dirs, errors.Join(errs...)
}

func safariPaths(safariBrowser browser.Browser, options browser.DiscoverOptions) []string {
	if options.ProfilePath != "" {
		return []string{filepath.Clean(options.ProfilePath)}
	}
	paths := make([]string, 0, len(safariBrowser.Platform().UserDataDirs))
	for _, relativePath := range safariBrowser.Platform().UserDataDirs {
		paths = append(paths, filepath.Join(options.Home, filepath.FromSlash(relativePath)))
	}
	return paths
}

// Extractors builds a named profile's file specs from Profile.Dir; its files sit under two dirs named for its UUID.
// The keychain, Bookmarks.plist, and the extension plists carry no profile tag, so only the default profile reads them.
func Extractors(dir browser.UserDataDir, profile browser.Profile, loginKeychain LoginKeychain) browser.Extractors {
	library := func(relativePath string) string { return fromLibrary(profile, relativePath) }
	uuid, named := profileUUID(profile)
	if !named {
		return browser.Extractors{
			category.Password: {Extract: passwordsFrom(loginKeychain)},
			category.History:  newPlainExtractor(newFileSpec("History.db"), extractHistories),
			category.Cookie: newPlainExtractor(newFileSpec(
				library(containerDir+"/Cookies/Cookies.binarycookies"),
				library("Cookies/Cookies.binarycookies"),
			), extractCookies),
			category.Bookmark:     newPlainExtractor(newFileSpec("Bookmarks.plist"), extractBookmarks),
			category.Download:     newPlainExtractor(newFileSpec("Downloads.plist"), downloadsOf(defaultProfileSentinel)),
			category.LocalStorage: newPlainExtractor(newDirectorySpec(library(containerDir+"/WebKit/WebsiteData/Default")), extractLocalStorage),
			category.Extension:    extensionsFrom(library),
		}
	}
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(uuid)
	return browser.Extractors{
		category.History:      newPlainExtractor(newFileSpec("History.db"), extractHistories),
		category.Cookie:       newPlainExtractor(newFileSpec(library(store+"/Cookies/Cookies.binarycookies")), extractCookies),
		category.Download:     newPlainExtractor(newFileSpec(library(safariDir(dir)+"/Downloads.plist")), downloadsOf(uuid)),
		category.LocalStorage: newPlainExtractor(newDirectorySpec(library(store+"/Origins")), extractLocalStorage),
	}
}

func profileUUID(profile browser.Profile) (string, bool) {
	if path.Dir(profile.Dir) != namedProfilesDir {
		return "", false
	}
	return path.Base(profile.Dir), true
}

// safariDir is the default profile's Dir, which holds the Downloads.plist every profile reads its entries from.
func safariDir(dir browser.UserDataDir) string {
	for _, profile := range dir.Profiles {
		if _, named := profileUUID(profile); !named {
			return profile.Dir
		}
	}
	return "Safari"
}

// fromLibrary turns a path under ~/Library into one relative to profile.Dir, which file specs are joined with.
func fromLibrary(profile browser.Profile, relativePath string) string {
	if profile.Dir == "" {
		return relativePath
	}
	return strings.Repeat("../", strings.Count(profile.Dir, "/")+1) + relativePath
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

// passwordsFrom reads no file, so it always runs; without a usable password the locked keychain yields metadata only.
func passwordsFrom(loginKeychain LoginKeychain) func(context.Context, browser.ProfileSnapshot, browser.Collector) error {
	return func(ctx context.Context, _ browser.ProfileSnapshot, out browser.Collector) error {
		if loginKeychain == nil {
			return errNoKeychain
		}
		return extractPasswords(ctx, loginKeychain, out)
	}
}

func downloadsOf(owner string) func(string, browser.Collector) error {
	return func(path string, out browser.Collector) error { return extractDownloads(path, owner, out) }
}

func extensionsFrom(library func(string) string) browser.Extractor {
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

// emit hands rows to out by pointer, because out sets each record's Browser and Profile in place.
func emit[T any, P interface {
	*T
	record.Record
}](out browser.Collector, rows []T) {
	for i := range rows {
		out.Add(P(&rows[i]))
	}
}
