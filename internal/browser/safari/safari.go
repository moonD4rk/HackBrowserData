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

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/keychain"
)

// Paths below are relative to the user data dir, ~/Library.
const (
	containerDir     = "Containers/com.apple.Safari/Data/Library"
	namedProfilesDir = containerDir + "/Safari/Profiles"

	defaultProfileName = "default"
)

var errNoKeychain = errors.New("no login keychain")

// Discover makes the Safari directory the default profile; -p names another, whose parent replaces ~/Library.
func Discover(ctx context.Context, safariBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
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
		id := safariBrowser.UserDataDirID(i)
		defaultProfile := browser.Profile{Name: defaultProfileName, RelativePath: filepath.Base(safariPath)}
		profiles := append([]browser.Profile{defaultProfile}, namedProfiles(ctx, library, id)...)
		disambiguateNames(profiles)
		dirs = append(dirs, browser.UserDataDir{ID: id, Browser: safariBrowser, Path: library, Profiles: profiles})
	}
	return dirs, errors.Join(errs...)
}

func userDataDirPaths(safariBrowser browser.Browser, options browser.DiscoverOptions) []string {
	if options.ProfilePath != "" {
		return []string{filepath.Clean(options.ProfilePath)}
	}
	paths := make([]string, 0, len(safariBrowser.Platform().UserDataDirPaths))
	for _, relativePath := range safariBrowser.Platform().UserDataDirPaths {
		paths = append(paths, filepath.Join(options.HomeDir, filepath.FromSlash(relativePath)))
	}
	return paths
}

// LoginKeychain is what Safari's password extraction reads from the run's macOS login keychain.
type LoginKeychain interface {
	InternetPasswords(ctx context.Context) ([]keychain.InternetPassword, error)
}

// extractor is how one category is read: the files it needs, and how it parses them; passwords come from the keychain.
type extractor struct {
	sources []browser.Source
	extract func(ctx context.Context, captured browser.ProfileSnapshot, loginKeychain LoginKeychain, out browser.Collector) error
}

// Sources lists the files of each category a profile supports; a named profile supports fewer than the default one.
func Sources(dir browser.UserDataDir, profile browser.Profile) map[category.Category][]browser.Source {
	extractors := profileExtractors(dir, profile)
	sources := make(map[category.Category][]browser.Source, len(extractors))
	for dataCategory, extractor := range extractors {
		sources[dataCategory] = extractor.sources
	}
	return sources
}

// Extract reads passwords from loginKeychain, which is nil off macOS; locked, it yields metadata only.
func Extract(
	ctx context.Context, captured browser.ProfileSnapshot, loginKeychain LoginKeychain, dataCategory category.Category, out browser.Collector,
) error {
	extractor, ok := profileExtractors(captured.UserDataDir, captured.Profile)[dataCategory]
	if !ok {
		return fmt.Errorf("%s profile %s has no %s extractor", captured.UserDataDir.Browser.Name, captured.Profile.Name, dataCategory)
	}
	return extractor.extract(ctx, captured, loginKeychain, out)
}

// profileExtractors builds a named profile's sources from Profile.RelativePath; its files sit under two dirs named for
// its UUID. The keychain, Bookmarks.plist, and the extension plists carry no profile tag, so only the default profile
// reads them.
func profileExtractors(dir browser.UserDataDir, profile browser.Profile) map[category.Category]extractor {
	if uuid, named := namedProfileUUID(profile); named {
		return namedProfileExtractors(dir, uuid)
	}
	return defaultProfileExtractors()
}

func defaultProfileExtractors() map[category.Category]extractor {
	var (
		history = browser.Source{RelativePath: "History.db"}
		cookies = browser.Source{
			RelativePath:    containerDir + "/Cookies/Cookies.binarycookies",
			Fallbacks:       []string{"Cookies/Cookies.binarycookies"},
			FromUserDataDir: true,
		}
		bookmarks     = browser.Source{RelativePath: "Bookmarks.plist"}
		downloads     = browser.Source{RelativePath: "Downloads.plist"}
		localStorage  = browser.Source{RelativePath: containerDir + "/WebKit/WebsiteData/Default", IsDir: true, FromUserDataDir: true}
		appExtensions = browser.Source{
			RelativePath: containerDir + "/Safari/AppExtensions/Extensions.plist", FromUserDataDir: true, Optional: true,
		}
		webExtensions = browser.Source{
			RelativePath: containerDir + "/Safari/WebExtensions/Extensions.plist", FromUserDataDir: true, Optional: true,
		}
	)
	return map[category.Category]extractor{
		category.Password:     {extract: extractPasswords},
		category.History:      newPlainExtractor(extractHistories, history),
		category.Cookie:       newPlainExtractor(extractCookies, cookies),
		category.Bookmark:     newPlainExtractor(extractBookmarks, bookmarks),
		category.Download:     newDownloadExtractor(downloads, defaultProfileSentinel),
		category.LocalStorage: newPlainExtractor(extractLocalStorage, localStorage),
		category.Extension:    newExtensionExtractor(appExtensions, webExtensions),
	}
}

func namedProfileExtractors(dir browser.UserDataDir, uuid string) map[category.Category]extractor {
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(uuid)
	var (
		history      = browser.Source{RelativePath: "History.db"}
		cookies      = browser.Source{RelativePath: store + "/Cookies/Cookies.binarycookies", FromUserDataDir: true}
		downloads    = browser.Source{RelativePath: defaultProfileDir(dir) + "/Downloads.plist", FromUserDataDir: true}
		localStorage = browser.Source{RelativePath: store + "/Origins", IsDir: true, FromUserDataDir: true}
	)
	return map[category.Category]extractor{
		category.History:      newPlainExtractor(extractHistories, history),
		category.Cookie:       newPlainExtractor(extractCookies, cookies),
		category.Download:     newDownloadExtractor(downloads, uuid),
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

func newPlainExtractor(extractFile func(string, browser.Collector) error, source browser.Source) extractor {
	return extractor{
		sources: []browser.Source{source},
		extract: func(_ context.Context, captured browser.ProfileSnapshot, _ LoginKeychain, out browser.Collector) error {
			capturedPath, err := captured.Path(source)
			if err != nil {
				return err
			}
			return extractFile(capturedPath, out)
		},
	}
}

// newDownloadExtractor keeps one profile's entries, because Downloads.plist holds every profile's.
func newDownloadExtractor(source browser.Source, profileUUID string) extractor {
	return newPlainExtractor(func(path string, out browser.Collector) error {
		return extractDownloads(path, profileUUID, out)
	}, source)
}

// newExtensionExtractor merges every plist, so the extensions come out sorted across both stores.
func newExtensionExtractor(sources ...browser.Source) extractor {
	return extractor{
		sources: sources,
		extract: func(_ context.Context, captured browser.ProfileSnapshot, _ LoginKeychain, out browser.Collector) error {
			var paths []string
			for _, source := range sources {
				capturedPath, err := captured.Path(source)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					return err
				}
				paths = append(paths, capturedPath)
			}
			return extractExtensions(paths, out)
		},
	}
}
