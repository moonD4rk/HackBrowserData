package firefox

import (
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser is one Firefox installation: the Profiles directory holding one or
// more profiles. Firefox keys are per-profile (each profile's key4.db), so the
// installation does not implement KeyManager.
type Browser struct {
	cfg      types.BrowserConfig
	profiles []*profile
}

// NewBrowser discovers the Firefox profiles under cfg.UserDataDir and returns
// the installation, or nil if no profile with resolvable sources exists.
// Firefox profile directories have random names (e.g. "97nszz88.default-release");
// any subdirectory containing known data files is treated as a valid profile.
func NewBrowser(cfg types.BrowserConfig) (*Browser, error) {
	var profiles []*profile
	for _, profileDir := range discoverProfiles(cfg.UserDataDir, firefoxSources) {
		sourcePaths := resolveSourcePaths(firefoxSources, profileDir)
		if len(sourcePaths) == 0 {
			continue
		}
		profiles = append(profiles, &profile{
			profileDir:  profileDir,
			browserName: cfg.Name,
			sourcePaths: sourcePaths,
		})
	}
	if len(profiles) == 0 {
		return nil, nil
	}
	return &Browser{cfg: cfg, profiles: profiles}, nil
}

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) UserDataDir() string { return b.cfg.UserDataDir }

// Profiles returns the identity of every profile in this installation.
func (b *Browser) Profiles() []types.Profile {
	out := make([]types.Profile, 0, len(b.profiles))
	for _, p := range b.profiles {
		out = append(out, types.Profile{Name: p.name(), Dir: p.profileDir})
	}
	return out
}

// Extract extracts every profile, deriving each profile's key independently.
func (b *Browser) Extract(categories []category.Category) ([]types.ExtractResult, error) {
	results := make([]types.ExtractResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.ExtractResult{
			Profile: types.Profile{Name: p.name(), Dir: p.profileDir},
			Data:    p.extract(categories),
		})
	}
	return results, nil
}

// CountEntries counts entries per category for every profile without decryption.
func (b *Browser) CountEntries(categories []category.Category) ([]types.CountResult, error) {
	results := make([]types.CountResult, 0, len(b.profiles))
	for _, p := range b.profiles {
		results = append(results, types.CountResult{
			Profile: types.Profile{Name: p.name(), Dir: p.profileDir},
			Counts:  p.count(categories),
		})
	}
	return results, nil
}

// resolvedPath holds the absolute path and type for a discovered source.
type resolvedPath struct {
	absPath string
	isDir   bool
}

// discoverProfiles lists subdirectories of userDataDir that contain at least
// one known data source. Each such directory is a Firefox profile.
func discoverProfiles(userDataDir string, sources map[category.Category][]sourcePath) []string {
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		return nil
	}

	var profiles []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(userDataDir, e.Name())
		if hasAnySource(sources, dir) {
			profiles = append(profiles, dir)
		}
	}
	return profiles
}

// hasAnySource checks if dir contains at least one source file or directory.
func hasAnySource(sources map[category.Category][]sourcePath, dir string) bool {
	for _, candidates := range sources {
		for _, sp := range candidates {
			abs := filepath.Join(dir, sp.rel)
			if _, err := os.Stat(abs); err == nil {
				return true
			}
		}
	}
	return false
}

// resolveSourcePaths checks which sources actually exist in profileDir.
// Candidates are tried in priority order; the first existing path wins.
func resolveSourcePaths(sources map[category.Category][]sourcePath, profileDir string) map[category.Category]resolvedPath {
	resolved := make(map[category.Category]resolvedPath)
	for cat, candidates := range sources {
		for _, sp := range candidates {
			abs := filepath.Join(profileDir, sp.rel)
			info, err := os.Stat(abs)
			if err != nil {
				continue
			}
			if sp.isDir == info.IsDir() {
				resolved[cat] = resolvedPath{abs, sp.isDir}
				break
			}
		}
	}
	return resolved
}
