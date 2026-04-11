package safari

import (
	"os"
	"path/filepath"
	"time"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Browser represents Safari browser data ready for extraction.
// Safari has a single flat data directory (no profile subdirectories)
// and stores most data unencrypted (passwords live in macOS Keychain).
type Browser struct {
	cfg         types.BrowserConfig
	dataDir     string                          // absolute path to ~/Library/Safari
	sources     map[types.Category][]sourcePath // Category → candidate paths
	sourcePaths map[types.Category]resolvedPath // Category → discovered absolute path
}

// NewBrowsers checks whether Safari data exists at cfg.UserDataDir and returns
// a single Browser if any known source files are found. Unlike Chromium/Firefox,
// Safari has no profile directories — the data directory is used directly.
func NewBrowsers(cfg types.BrowserConfig) ([]*Browser, error) {
	sourcePaths := resolveSourcePaths(safariSources, cfg.UserDataDir)
	if len(sourcePaths) == 0 {
		return nil, nil
	}
	return []*Browser{{
		cfg:         cfg,
		dataDir:     cfg.UserDataDir,
		sources:     safariSources,
		sourcePaths: sourcePaths,
	}}, nil
}

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) ProfileDir() string  { return b.dataDir }
func (b *Browser) ProfileName() string { return "default" }

// Extract copies browser files to a temp directory and extracts data
// for the requested categories.
func (b *Browser) Extract(categories []types.Category) (*types.BrowserData, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	data := &types.BrowserData{}
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		b.extractCategory(data, cat, path)
	}
	return data, nil
}

// CountEntries copies browser files to a temp directory and counts entries
// per category without full extraction.
func (b *Browser) CountEntries(categories []types.Category) (map[types.Category]int, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	counts := make(map[types.Category]int)
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		counts[cat] = b.countCategory(cat, path)
	}
	return counts, nil
}

// acquireFiles copies source files to the session temp directory.
func (b *Browser) acquireFiles(session *filemanager.Session, categories []types.Category) map[types.Category]string {
	tempPaths := make(map[types.Category]string)
	for _, cat := range categories {
		rp, ok := b.sourcePaths[cat]
		if !ok {
			continue
		}
		dst := filepath.Join(session.TempDir(), cat.String())
		if err := session.Acquire(rp.absPath, dst, rp.isDir); err != nil {
			log.Debugf("acquire %s: %v", cat, err)
			continue
		}
		tempPaths[cat] = dst
	}
	return tempPaths
}

// extractCategory calls the appropriate extract function for a category.
func (b *Browser) extractCategory(data *types.BrowserData, cat types.Category, path string) {
	var err error
	switch cat {
	case types.History:
		data.Histories, err = extractHistories(path)
	case types.Cookie:
		data.Cookies, err = extractCookies(path)
	case types.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case types.Download:
		data.Downloads, err = extractDownloads(path)
	default:
		return
	}
	if err != nil {
		log.Debugf("extract %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
}

// countCategory calls the appropriate count function for a category.
func (b *Browser) countCategory(cat types.Category, path string) int {
	var count int
	var err error
	switch cat {
	case types.History:
		count, err = countHistories(path)
	case types.Cookie:
		count, err = countCookies(path)
	case types.Bookmark:
		count, err = countBookmarks(path)
	case types.Download:
		count, err = countDownloads(path)
	default:
		// Unsupported categories silently return 0.
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
	return count
}

// resolvedPath holds the absolute path and type for a discovered source.
type resolvedPath struct {
	absPath string
	isDir   bool
}

// resolveSourcePaths checks which sources actually exist in dataDir.
// Candidates are tried in priority order; the first existing path wins.
func resolveSourcePaths(sources map[types.Category][]sourcePath, dataDir string) map[types.Category]resolvedPath {
	resolved := make(map[types.Category]resolvedPath)
	for cat, candidates := range sources {
		for _, sp := range candidates {
			abs := filepath.Join(dataDir, sp.rel)
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

// coreDataEpochOffset is the number of seconds between the Unix epoch
// (1970-01-01) and the Core Data epoch (2001-01-01).
const coreDataEpochOffset = 978307200

// coredataTimestamp converts a Core Data timestamp (seconds since 2001-01-01)
// to a time.Time. Safari's History.db uses this epoch for visit_time.
func coredataTimestamp(seconds float64) time.Time {
	return time.Unix(int64(seconds)+coreDataEpochOffset, 0)
}
