// Package chromium is the Chromium base: profile discovery, file parsing, and where a user data dir's keys come from.
package chromium

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// profileMarkers add Preferences_02 for Sogou and QQ; a marker keeps dirs like Crashpad from passing as profiles.
var profileMarkers = []string{"Preferences", "Preferences_02"}

// Discover yields nothing for a missing user data dir; any other failure is returned beside whatever was found.
func Discover(chromiumBrowser browser.Browser, options browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	if dir, ok := profileAt(chromiumBrowser, options.ProfilePath); ok {
		return []browser.UserDataDir{dir}, nil
	}
	paths, err := userDataDirPaths(chromiumBrowser, options)
	errs := []error{err}
	var dirs []browser.UserDataDir
	for i, path := range paths {
		dir := browser.UserDataDir{ID: chromiumBrowser.UserDataDirID(i), Browser: chromiumBrowser, Path: path}
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

// DiscoverAt reads the profiles of a user data dir the table does not name, such as a copy restore was given.
func DiscoverAt(chromiumBrowser browser.Browser, id browser.UserDataDirID, path string) (browser.UserDataDir, error) {
	dir := browser.UserDataDir{ID: id, Browser: chromiumBrowser, Path: path}
	profiles, err := discoverProfiles(dir)
	if err != nil {
		return dir, err
	}
	dir.Profiles = profiles
	return dir, nil
}

// profileAt takes chrome://version's path: a profile dir without Local State selects that profile of its parent.
func profileAt(chromiumBrowser browser.Browser, path string) (browser.UserDataDir, bool) {
	if path == "" || !isProfileDir(path) || hasAnyFile(path, localState.Paths()) {
		return browser.UserDataDir{}, false
	}
	clean := filepath.Clean(path)
	name := filepath.Base(clean)
	return browser.UserDataDir{
		ID:       chromiumBrowser.UserDataDirID(0),
		Browser:  chromiumBrowser,
		Path:     filepath.Dir(clean),
		Profiles: []browser.Profile{{Name: name, RelativePath: name}},
	}, true
}

// userDataDirPaths keeps every glob match, for both Arc packages; "/" joins keep the paths keys.json recorded.
func userDataDirPaths(chromiumBrowser browser.Browser, options browser.DiscoverOptions) ([]string, error) {
	if options.ProfilePath != "" {
		return []string{options.ProfilePath}, nil
	}
	var (
		paths []string
		errs  []error
	)
	for _, relativePath := range chromiumBrowser.Platform().UserDataDirs {
		matches, err := filepath.Glob(options.HomeDir + "/" + relativePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("user data dir %q: %w", relativePath, err))
			continue
		}
		paths = append(paths, matches...)
	}
	return paths, errors.Join(errs...)
}

func discoverProfiles(dir browser.UserDataDir) ([]browser.Profile, error) {
	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		return nil, err
	}

	var profiles []browser.Profile
	for _, entry := range entries {
		if entry.IsDir() && !isSkippedProfileDir(entry.Name()) && isProfileDir(filepath.Join(dir.Path, entry.Name())) {
			profiles = append(profiles, browser.Profile{Name: entry.Name(), RelativePath: entry.Name()})
		}
	}
	if len(profiles) > 0 {
		return profiles, nil
	}

	// Flat layout (older Opera): check the root before subdirs, so a stray subdir holding profile files can't hide it.
	profileFiles := profileFilePaths(dir)
	if hasAnyFile(dir.Path, profileFiles) {
		return []browser.Profile{{Name: filepath.Base(dir.Path)}}, nil
	}

	// Copied or restored trees may lack the marker, which no category reads, so a subdir with profile files is one too.
	for _, entry := range entries {
		if entry.IsDir() && !isSkippedProfileDir(entry.Name()) && hasAnyFile(filepath.Join(dir.Path, entry.Name()), profileFiles) {
			profiles = append(profiles, browser.Profile{Name: entry.Name(), RelativePath: entry.Name()})
		}
	}
	return profiles, nil
}

func isProfileDir(path string) bool {
	for _, name := range profileMarkers {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			return true
		}
	}
	return false
}

func isSkippedProfileDir(name string) bool {
	switch name {
	case "System Profile", "Guest Profile", "Snapshot":
		return true
	}
	return false
}

func profileFilePaths(dir browser.UserDataDir) []string {
	var paths []string
	for _, extractor := range Extractors(dir, browser.Profile{}) {
		for _, source := range extractor.Sources {
			if !source.Shared {
				paths = append(paths, source.Paths()...)
			}
		}
	}
	return paths
}

func hasAnyFile(path string, relativePaths []string) bool {
	for _, relativePath := range relativePaths {
		if _, err := os.Stat(filepath.Join(path, filepath.FromSlash(relativePath))); err == nil {
			return true
		}
	}
	return false
}

// KeyLocator leaves the run's login keychain to the caller, which adds it to the Locator on macOS.
func KeyLocator(dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) (masterkey.Locator, error) {
	platform := dir.Browser.Platform()
	locator := masterkey.Locator{
		SecretLabel: platform.SecretLabel, Executable: platform.Executable, ExecutableFallbacks: platform.ExecutableFallbacks, ABE: platform.ABE,
	}
	path, err := dirSnapshot.Path(localState.Path)
	switch {
	case err == nil:
		locator.LocalState = path
	case !errors.Is(err, fs.ErrNotExist):
		return locator, fmt.Errorf("local state: %w", err)
	}
	return locator, nil
}

// KeySources are the files KeyLocator reads from a snapshot.
func KeySources() []browser.Source {
	return []browser.Source{localState}
}

// ArchiveSources add each profile's marker, by which restore rediscovers the profiles, to the key files.
func ArchiveSources() []browser.Source {
	sources := KeySources()
	for _, marker := range profileMarkers {
		sources = append(sources, browser.Source{Path: marker, Optional: true})
	}
	return sources
}
