package safari

import (
	"os"
	"time"

	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser is one Safari installation, holding the default profile and any named
// profiles. Passwords come from the shared macOS Keychain; the login password is
// set on the installation and threaded to each profile at extract time.
type Browser struct {
	cfg              types.BrowserConfig
	keychainPassword string
	profiles         []*profile
}

// SetKeychainPassword sets the macOS login password used to unlock the Keychain.
func (b *Browser) SetKeychainPassword(password string) { b.keychainPassword = password }

// NewBrowser returns the Safari installation with one profile per Safari profile
// that has resolvable data, or nil if none. Named profiles are enumerated from
// SafariTabs.db.
func NewBrowser(cfg types.BrowserConfig) (*Browser, error) {
	var profiles []*profile
	for _, p := range discoverSafariProfiles(cfg.UserDataDir) {
		paths := resolveProfilePaths(p)
		if len(paths) == 0 {
			continue
		}
		profiles = append(profiles, &profile{
			ctx:         p,
			browserName: cfg.Name,
			sourcePaths: paths,
		})
	}
	if len(profiles) == 0 {
		return nil, nil
	}
	return &Browser{cfg: cfg, profiles: profiles}, nil
}

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) UserDataDir() string { return b.cfg.UserDataDir }

// Profiles returns the identity of every Safari profile in this installation.
func (b *Browser) Profiles() []types.Profile {
	out := make([]types.Profile, 0, len(b.profiles))
	for _, p := range b.profiles {
		out = append(out, types.Profile{Name: p.ctx.name, Dir: p.dir()})
	}
	return out
}

// Extract extracts every profile, threading the installation's keychain password.
func (b *Browser) Extract(categories []types.Category) ([]types.ExtractResult, error) {
	results := make([]types.ExtractResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.ExtractResult{
			Profile: types.Profile{Name: p.ctx.name, Dir: p.dir()},
			Data:    p.extract(categories, b.keychainPassword),
		})
	}
	return results, nil
}

// CountEntries counts entries per category for every profile.
func (b *Browser) CountEntries(categories []types.Category) ([]types.CountResult, error) {
	results := make([]types.CountResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.CountResult{
			Profile: types.Profile{Name: p.ctx.name, Dir: p.dir()},
			Counts:  p.count(categories, b.keychainPassword),
		})
	}
	return results, nil
}

func resolveProfilePaths(p profileContext) map[types.Category]resolvedPath {
	return resolveSourcePaths(buildSources(p))
}

type resolvedPath struct {
	absPath string
	isDir   bool
}

// resolveSourcePaths returns only paths that exist; first matching candidate wins per category.
func resolveSourcePaths(sources map[types.Category][]sourcePath) map[types.Category]resolvedPath {
	resolved := make(map[types.Category]resolvedPath)
	for cat, candidates := range sources {
		for _, sp := range candidates {
			info, err := os.Stat(sp.abs)
			if err != nil {
				continue
			}
			if sp.isDir == info.IsDir() {
				resolved[cat] = resolvedPath{sp.abs, sp.isDir}
				break
			}
		}
	}
	return resolved
}

// Offset from the Core Data epoch (2001-01-01 UTC) to the Unix epoch.
const coreDataEpochOffset = 978307200

// maxCoreDataSeconds is the largest CFAbsoluteTime that still lands inside
// time.Time.MarshalJSON's [1, 9999] year window. Also bounds the float →
// int64 conversion below; Go's spec makes out-of-range conversions return
// an implementation-dependent int64, which could silently corrupt results.
const maxCoreDataSeconds = 252423993600

// coredataTimestamp converts Core Data seconds (CFAbsoluteTime) to UTC.
// Returns zero for non-positive input or out-of-JSON-range values.
func coredataTimestamp(seconds float64) time.Time {
	if seconds <= 0 || seconds > maxCoreDataSeconds {
		return time.Time{}
	}
	whole := int64(seconds)
	frac := seconds - float64(whole)
	nanos := int64(frac * 1e9)
	return time.Unix(whole+coreDataEpochOffset, nanos).UTC()
}
