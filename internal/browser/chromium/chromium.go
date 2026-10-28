// Package chromium is the Chromium base: where Chromium-family browsers keep their profiles, how their files are
// parsed, and where a user data dir's keys come from.
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

// profileMarkers name the per-profile preferences file Chromium writes on first use; QQ Browser and Sogou Explorer write
// Preferences_02. The marker keeps Crashpad, ShaderCache, and the like from passing as profiles.
var profileMarkers = []string{"Preferences", "Preferences_02"}

// Discover finds def's user data dirs and their profiles. A user data dir that does not exist yields nothing; any
// other failure is returned, joined across dirs, beside whatever was found.
func Discover(def browser.Def, opts browser.DiscoverOptions) ([]browser.UserDataDir, error) {
	if dir, ok := profileAt(def, opts.ProfilePath); ok {
		return []browser.UserDataDir{dir}, nil
	}
	paths, err := userDataDirPaths(def, opts)
	errs := []error{err}
	var dirs []browser.UserDataDir
	for i, path := range paths {
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

// DiscoverAt reads the profiles of a user data dir the table does not name, such as a copy restore was given.
func DiscoverAt(def browser.Def, id browser.DirID, path string) (browser.UserDataDir, error) {
	dir := browser.UserDataDir{ID: id, Def: def, Path: path}
	profiles, err := discoverProfiles(dir)
	if err != nil {
		return dir, err
	}
	dir.Profiles = profiles
	return dir, nil
}

// profileAt reads -p as the path chrome://version shows: a directory with a profile marker but no Local State is one
// profile, so its parent is the user data dir that holds the keys, and only that profile is selected.
func profileAt(def browser.Def, path string) (browser.UserDataDir, bool) {
	if path == "" || !isProfileDir(path) || hasAnySource(path, localState.Paths) {
		return browser.UserDataDir{}, false
	}
	clean := filepath.Clean(path)
	name := filepath.Base(clean)
	return browser.UserDataDir{
		ID:       dirID(def, 0),
		Def:      def,
		Path:     filepath.Dir(clean),
		Profiles: []browser.Profile{{Name: name, Dir: name}},
	}, true
}

// userDataDirPaths keeps every glob match, so both of a machine's Arc packages are found. Joining with "/" keeps the
// path strings keys.json has always recorded; -p replaces the table's entries.
func userDataDirPaths(def browser.Def, opts browser.DiscoverOptions) ([]string, error) {
	if opts.ProfilePath != "" {
		return []string{opts.ProfilePath}, nil
	}
	var (
		paths []string
		errs  []error
	)
	for _, rel := range def.Platform().UserDataDirs {
		matches, err := filepath.Glob(opts.Home + "/" + rel)
		if err != nil {
			errs = append(errs, fmt.Errorf("user data dir %q: %w", rel, err))
			continue
		}
		paths = append(paths, matches...)
	}
	return paths, errors.Join(errs...)
}

// dirID numbers by path order among the matches, so IDs stay the same from run to run.
func dirID(def browser.Def, i int) browser.DirID {
	if i == 0 {
		return browser.DirID(def.ID)
	}
	return browser.DirID(fmt.Sprintf("%s-%d", def.ID, i+1))
}

func discoverProfiles(dir browser.UserDataDir) ([]browser.Profile, error) {
	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		return nil, err
	}

	var profiles []browser.Profile
	for _, e := range entries {
		if e.IsDir() && !isSkippedDir(e.Name()) && isProfileDir(filepath.Join(dir.Path, e.Name())) {
			profiles = append(profiles, browser.Profile{Name: e.Name(), Dir: e.Name()})
		}
	}
	if len(profiles) > 0 {
		return profiles, nil
	}

	// Flat layout (older Opera): data files directly under the user data dir. Check the root before the subdir fallback
	// so a stray source-bearing subdir can't suppress root discovery.
	sources := profileSourcePaths(dir)
	if hasAnySource(dir.Path, sources) {
		return []browser.Profile{{Name: filepath.Base(dir.Path)}}, nil
	}

	// Restored or copied trees may omit the marker, which no category reads, so any source-bearing subdir is a profile.
	for _, e := range entries {
		if e.IsDir() && !isSkippedDir(e.Name()) && hasAnySource(filepath.Join(dir.Path, e.Name()), sources) {
			profiles = append(profiles, browser.Profile{Name: e.Name(), Dir: e.Name()})
		}
	}
	return profiles, nil
}

func isProfileDir(dir string) bool {
	for _, name := range profileMarkers {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func isSkippedDir(name string) bool {
	switch name {
	case "System Profile", "Guest Profile", "Snapshot":
		return true
	}
	return false
}

func profileSourcePaths(dir browser.UserDataDir) []string {
	var paths []string
	for _, e := range Extractors(dir, browser.Profile{}) {
		for _, src := range e.Sources {
			if !src.Shared {
				paths = append(paths, src.Paths...)
			}
		}
	}
	return paths
}

func hasAnySource(dir string, paths []string) bool {
	for _, rel := range paths {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
			return true
		}
	}
	return false
}

// KeyLocator says where the user data dir's keys are: the Local State in snap and its browser's OS fields. On macOS
// the caller adds the run's login keychain.
func KeyLocator(dir browser.UserDataDir, snap snapshot.Snapshot) (masterkey.Locator, error) {
	p := dir.Def.Platform()
	loc := masterkey.Locator{KeychainLabel: p.KeychainLabel, Exe: p.Exe, ExeFallbacks: p.ExeFallbacks, ABE: p.ABE}
	path, err := snap.Path(localState.Paths[0])
	switch {
	case err == nil:
		loc.LocalState = path
	case !errors.Is(err, fs.ErrNotExist):
		return loc, fmt.Errorf("local state: %w", err)
	}
	return loc, nil
}

// KeySources are the files KeyLocator reads from a snapshot.
func KeySources(_ browser.UserDataDir) []browser.Source {
	return []browser.Source{localState}
}

// ArchiveSources are what an archive keeps beside the categories' own files: Local State, and each profile's marker so
// that a reader without the manifest still finds every profile.
func ArchiveSources(_ browser.UserDataDir) []browser.Source {
	sources := []browser.Source{localState}
	for _, marker := range profileMarkers {
		sources = append(sources, browser.Source{Paths: []string{marker}, Optional: true})
	}
	return sources
}
