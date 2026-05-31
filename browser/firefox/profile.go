package firefox

import (
	"fmt"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// profile is one Firefox profile — the leaf extraction unit. Unlike Chromium,
// each Firefox profile owns its own master key (derived from its key4.db).
type profile struct {
	profileDir  string
	browserName string
	sourcePaths map[types.Category]resolvedPath
}

func (p *profile) name() string {
	if p.profileDir == "" {
		return ""
	}
	return filepath.Base(p.profileDir)
}

func (p *profile) label() string { return p.browserName + "/" + p.name() }

// extract copies the profile's source files to a temp directory, derives the
// per-profile master key, and extracts the requested categories.
func (p *profile) extract(categories []types.Category) *types.BrowserData {
	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return &types.BrowserData{}
	}
	defer session.Cleanup()

	tempPaths := p.acquireFiles(session, categories)

	masterKey, err := p.getMasterKey(session, tempPaths)
	if err != nil {
		log.Debugf("get master key for %s: %v", p.label(), err)
	}

	data := &types.BrowserData{}
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		p.extractCategory(data, cat, masterKey, path)
	}
	return data
}

// count counts entries per category without decryption.
func (p *profile) count(categories []types.Category) map[types.Category]int {
	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return nil
	}
	defer session.Cleanup()

	tempPaths := p.acquireFiles(session, categories)
	counts := make(map[types.Category]int)
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		counts[cat] = p.countCategory(cat, path)
	}
	return counts
}

// acquireFiles copies source files to the session temp directory.
func (p *profile) acquireFiles(session *filemanager.Session, categories []types.Category) map[types.Category]string {
	tempPaths := make(map[types.Category]string)
	for _, cat := range categories {
		rp, ok := p.sourcePaths[cat]
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

// getMasterKey retrieves the Firefox master encryption key from this profile's
// key4.db. The key is derived via NSS ASN1 PBE decryption (platform-agnostic).
// If logins.json was already acquired by acquireFiles, the derived key is
// validated by attempting to decrypt an actual login entry.
func (p *profile) getMasterKey(session *filemanager.Session, tempPaths map[types.Category]string) ([]byte, error) {
	key4Src := filepath.Join(p.profileDir, "key4.db")
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

// extractCategory calls the appropriate extract function for a category.
func (p *profile) extractCategory(data *types.BrowserData, cat types.Category, masterKey []byte, path string) {
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
		log.Debugf("extract %s for %s: %v", cat, p.label(), err)
	}
}

// countCategory calls the appropriate count function for a category.
func (p *profile) countCategory(cat types.Category, path string) int {
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
		log.Debugf("count %s for %s: %v", cat, p.label(), err)
	}
	return count
}
