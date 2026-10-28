// Package safari is the Safari base: where Safari keeps its default and named profiles under ~/Library, how their
// files are parsed, and the login keychain its passwords come from.
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

	defaultProfileName = "default"
	discoverDirID      = "safari"
)

var errNoKeychain = errors.New("no login keychain")

// LoginKeychain is the one method the password extractor reads from the run's login keychain.
type LoginKeychain interface {
	InternetPasswords(ctx context.Context) ([]keychainbreaker.InternetPassword, error)
}

// Discover finds Safari's data under ~/Library: the default profile, whose Dir is the Safari directory, and the named
// profiles SafariTabs.db lists. -p does not apply to Safari, and Validate rejects it at the switch-over.
func Discover(def browser.Def, opts browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	var errs []error
	var dirs []browser.UserDataDir
	for i, rel := range def.Platform().UserDataDirs {
		safariPath := filepath.Join(opts.Home, filepath.FromSlash(rel))
		if _, err := os.Stat(safariPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("%s: %w", safariPath, err))
			}
			continue
		}
		library := filepath.Dir(safariPath)
		profiles := append([]browser.Profile{{Name: defaultProfileName, Dir: filepath.Base(safariPath)}}, namedProfiles(library)...)
		disambiguateNames(profiles)
		dirs = append(dirs, browser.UserDataDir{ID: dirID(def, i), Def: def, Path: library, Profiles: profiles})
	}
	return dirs, errors.Join(errs...)
}

func dirID(def browser.Def, i int) browser.DirID {
	if i == 0 {
		return browser.DirID(def.ID)
	}
	return browser.DirID(fmt.Sprintf("%s-%d", def.ID, i+1))
}

// Extractors maps the categories a Safari profile supports to their extractors. A named profile's files sit under
// two directories named for its UUID, so its sources are built from Profile.Dir; the keychain, Bookmarks.plist, and
// the extension plists carry no profile tag, so only the default profile reads them.
func Extractors(dir browser.UserDataDir, p browser.Profile, kc LoginKeychain) browser.Extractors {
	library := func(rel string) string { return fromLibrary(p, rel) }
	uuid, named := profileUUID(p)
	if !named {
		return browser.Extractors{
			category.Password: {Extract: passwordsFrom(kc)},
			category.History:  plain(file("History.db"), extractHistories),
			category.Cookie: plain(file(
				library(containerDir+"/Cookies/Cookies.binarycookies"),
				library("Cookies/Cookies.binarycookies"),
			), extractCookies),
			category.Bookmark:     plain(file("Bookmarks.plist"), extractBookmarks),
			category.Download:     plain(file("Downloads.plist"), downloadsOf(defaultProfileSentinel)),
			category.LocalStorage: plain(directory(library(containerDir+"/WebKit/WebsiteData/Default")), extractLocalStorage),
			category.Extension:    extensionsFrom(library),
		}
	}
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(uuid)
	return browser.Extractors{
		category.History:      plain(file("History.db"), extractHistories),
		category.Cookie:       plain(file(library(store+"/Cookies/Cookies.binarycookies")), extractCookies),
		category.Download:     plain(file(library(safariDir(dir)+"/Downloads.plist")), downloadsOf(uuid)),
		category.LocalStorage: plain(directory(library(store+"/Origins")), extractLocalStorage),
	}
}

func profileUUID(p browser.Profile) (string, bool) {
	if path.Dir(p.Dir) != namedProfilesDir {
		return "", false
	}
	return path.Base(p.Dir), true
}

// safariDir is the default profile's Dir, which holds the Downloads.plist every profile reads its entries from.
func safariDir(dir browser.UserDataDir) string {
	for _, p := range dir.Profiles {
		if _, named := profileUUID(p); !named {
			return p.Dir
		}
	}
	return "Safari"
}

// fromLibrary turns a path under ~/Library into one relative to p.Dir, because a profile's sources are joined with it.
func fromLibrary(p browser.Profile, rel string) string {
	if p.Dir == "" {
		return rel
	}
	return strings.Repeat("../", strings.Count(p.Dir, "/")+1) + rel
}

func file(paths ...string) browser.Source { return browser.Source{Paths: paths} }
func directory(rel string) browser.Source { return browser.Source{Paths: []string{rel}, IsDir: true} }

func plain(src browser.Source, parse func(string, browser.Collector) error) browser.Extractor {
	return browser.Extractor{
		Sources: []browser.Source{src},
		Extract: func(_ context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
			captured, err := ps.Path(src)
			if err != nil {
				return err
			}
			return parse(captured, out)
		},
	}
}

// passwordsFrom reads no file, so the extraction always runs; the keychain stays locked without a usable password and
// yields metadata only.
func passwordsFrom(kc LoginKeychain) func(context.Context, browser.ProfileSnapshot, browser.Collector) error {
	return func(ctx context.Context, _ browser.ProfileSnapshot, out browser.Collector) error {
		if kc == nil {
			return errNoKeychain
		}
		return extractPasswords(ctx, kc, out)
	}
}

// downloadsOf keeps the entries of one profile: Downloads.plist is shared, and each entry names the profile it
// belongs to.
func downloadsOf(owner string) func(string, browser.Collector) error {
	return func(path string, out browser.Collector) error { return extractDownloads(path, owner, out) }
}

func extensionsFrom(library func(string) string) browser.Extractor {
	sources := []browser.Source{
		{Paths: []string{library(containerDir + "/Safari/AppExtensions/Extensions.plist")}, Optional: true},
		{Paths: []string{library(containerDir + "/Safari/WebExtensions/Extensions.plist")}, Optional: true},
	}
	return browser.Extractor{
		Sources: sources,
		Extract: func(_ context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
			var paths []string
			for _, src := range sources {
				captured, err := ps.Path(src)
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

// emit hands rows to out by pointer, because out stamps each record in place.
func emit[T any, P interface {
	*T
	record.Record
}](out browser.Collector, rows []T) {
	for i := range rows {
		out.Add(P(&rows[i]))
	}
}
