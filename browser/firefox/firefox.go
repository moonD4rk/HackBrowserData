package firefox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// Browser represents a single Firefox profile ready for extraction.
type Browser struct {
	cfg         types.BrowserConfig
	profileDir  string                          // absolute path to profile directory
	sources     map[types.Category][]sourcePath // Category → candidate paths (priority order)
	sourcePaths map[types.Category]resolvedPath // Category → discovered absolute path
}

// NewBrowsers discovers Firefox profiles under cfg.UserDataDir and returns
// one Browser per profile. Firefox profile directories have random names
// (e.g. "97nszz88.default-release"); any subdirectory containing known
// data files is treated as a valid profile.
func NewBrowsers(cfg types.BrowserConfig) ([]*Browser, error) {
	profileDirs := discoverProfiles(cfg.UserDataDir, firefoxSources)
	if len(profileDirs) == 0 {
		return nil, nil
	}

	var browsers []*Browser
	for _, profileDir := range profileDirs {
		sourcePaths := resolveSourcePaths(firefoxSources, profileDir)
		if len(sourcePaths) == 0 {
			continue
		}
		browsers = append(browsers, &Browser{
			cfg:         cfg,
			profileDir:  profileDir,
			sources:     firefoxSources,
			sourcePaths: sourcePaths,
		})
	}
	return browsers, nil
}

func (b *Browser) BrowserName() string { return b.cfg.Name }
func (b *Browser) ProfileDir() string  { return b.profileDir }
func (b *Browser) ProfileName() string {
	if b.profileDir == "" {
		return ""
	}
	return filepath.Base(b.profileDir)
}

// Extract copies browser files to a temp directory, retrieves the master key,
// and extracts data for the requested categories.
func (b *Browser) Extract(categories []types.Category) (*types.BrowserData, error) {
	session, err := filemanager.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	tempPaths := b.acquireFiles(session, categories)

	masterKey, err := b.getMasterKey(session, tempPaths)
	if err != nil {
		log.Debugf("get master key for %s: %v", b.BrowserName()+"/"+b.ProfileName(), err)
	}

	data := &types.BrowserData{}
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		b.extractCategory(data, cat, masterKey, path)
	}
	return data, nil
}

// CountEntries copies browser files to a temp directory and counts entries
// per category without decryption. Much faster than Extract for display-only
// use cases like "list --detail".
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

// countCategory calls the appropriate count function for a category.
func (b *Browser) countCategory(cat types.Category, path string) int {
	var count int
	var err error
	switch cat {
	case types.Password:
		count, err = countPasswords(path)
	case types.Cookie:
		count, err = countCookies(path)
	case types.History:
		count, err = countHistories(path)
	case types.Download:
		count, err = countDownloads(path)
	case types.Bookmark:
		count, err = countBookmarks(path)
	case types.Extension:
		count, err = countExtensions(path)
	case types.LocalStorage:
		count, err = countLocalStorage(path)
	case types.CreditCard, types.SessionStorage:
		// Firefox does not support CreditCard or SessionStorage.
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
	return count
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

// getMasterKey retrieves the Firefox master encryption key from key4.db.
// The key is derived via NSS ASN1 PBE decryption (platform-agnostic).
// If logins.json was already acquired by acquireFiles, the derived key
// is validated by attempting to decrypt an actual login entry.
func (b *Browser) getMasterKey(session *filemanager.Session, tempPaths map[types.Category]string) ([]byte, error) {
	key4Src := filepath.Join(b.profileDir, "key4.db")
	if !fileutil.FileExists(key4Src) {
		return nil, nil
	}
	key4Dst := filepath.Join(session.TempDir(), "key4.db")
	if err := session.Acquire(key4Src, key4Dst, false); err != nil {
		return nil, fmt.Errorf("acquire key4.db: %w", err)
	}

	// logins.json is already acquired by acquireFiles as the Password source;
	// reuse it for master key validation if available.
	loginsPath := tempPaths[types.Password]
	return retrieveMasterKey(key4Dst, loginsPath)
}

// retrieveMasterKey reads key4.db and derives the master key using NSS.
// If loginsPath is non-empty, the derived key is validated against actual
// login data to ensure the correct candidate is selected.
func retrieveMasterKey(key4Path, loginsPath string) ([]byte, error) {
	k4, err := readKey4DB(key4Path)
	if err != nil {
		return nil, err
	}

	keys, err := k4.deriveKeys()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("no valid master key candidates in key4.db")
	}

	// No logins to validate against — return the first derived key.
	if loginsPath == "" {
		return keys[0], nil
	}

	// Validate against actual login data.
	if key := validateKeyWithLogins(keys, loginsPath); key != nil {
		return key, nil
	}

	return nil, fmt.Errorf("derived %d key(s) but none could decrypt logins", len(keys))
}

// extractCategory calls the appropriate extract function for a category.
func (b *Browser) extractCategory(data *types.BrowserData, cat types.Category, masterKey []byte, path string) {
	var err error
	switch cat {
	case types.Password:
		data.Passwords, err = extractPasswords(masterKey, path)
	case types.Cookie:
		data.Cookies, err = extractCookies(path)
	case types.History:
		data.Histories, err = extractHistories(path)
	case types.Download:
		data.Downloads, err = extractDownloads(path)
	case types.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case types.Extension:
		data.Extensions, err = extractExtensions(path)
	case types.LocalStorage:
		data.LocalStorage, err = extractLocalStorage(path)
	case types.CreditCard, types.SessionStorage:
		// Firefox does not support CreditCard or SessionStorage extraction.
	}
	if err != nil {
		log.Debugf("extract %s for %s: %v", cat, b.BrowserName()+"/"+b.ProfileName(), err)
	}
}

// resolvedPath holds the absolute path and type for a discovered source.
type resolvedPath struct {
	absPath string
	isDir   bool
}

// discoverProfiles lists subdirectories of userDataDir that contain at least
// one known data source. Each such directory is a Firefox profile.
func discoverProfiles(userDataDir string, sources map[types.Category][]sourcePath) []string {
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
func hasAnySource(sources map[types.Category][]sourcePath, dir string) bool {
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
func resolveSourcePaths(sources map[types.Category][]sourcePath, profileDir string) map[types.Category]resolvedPath {
	resolved := make(map[types.Category]resolvedPath)
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

// Firefox uses three timestamp units. Helpers emit UTC and return the zero
// time.Time for non-positive or out-of-JSON-range input.
//
//   - firefoxMicros: PRTime (μs since Unix epoch) — moz_* tables.
//   - firefoxMillis: Date.now() (ms) — logins.json, download endTime.
//   - firefoxSeconds: seconds — moz_cookies.expiry only.
func firefoxMicros(us int64) time.Time {
	if us <= 0 {
		return time.Time{}
	}
	return clampJSON(time.UnixMicro(us).UTC())
}

func firefoxMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return clampJSON(time.UnixMilli(ms).UTC())
}

func firefoxSeconds(s int64) time.Time {
	if s <= 0 {
		return time.Time{}
	}
	return clampJSON(time.Unix(s, 0).UTC())
}

// clampJSON maps years outside time.Time.MarshalJSON's [1, 9999] window
// to the zero time, so JSON export can't crash on sentinel inputs.
func clampJSON(t time.Time) time.Time {
	if t.Year() < 1 || t.Year() > 9999 {
		return time.Time{}
	}
	return t
}
