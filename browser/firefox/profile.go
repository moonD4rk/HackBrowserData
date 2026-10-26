package firefox

import (
	"fmt"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// profile is one Firefox profile — the leaf extraction unit. Unlike Chromium,
// each Firefox profile owns its own master key (derived from its key4.db).
type profile struct {
	profileDir  string
	browserName string
	sourcePaths map[category.Category]resolvedPath
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
func (p *profile) extract(categories []category.Category) *types.BrowserData {
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

func (p *profile) count(categories []category.Category) map[category.Category]int {
	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return nil
	}
	defer session.Cleanup()

	tempPaths := p.acquireFiles(session, categories)
	counts := make(map[category.Category]int)
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		counts[cat] = p.countCategory(cat, path)
	}
	return counts
}

func (p *profile) acquireFiles(session *filemanager.Session, categories []category.Category) map[category.Category]string {
	tempPaths := make(map[category.Category]string)
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
func (p *profile) getMasterKey(session *filemanager.Session, tempPaths map[category.Category]string) ([]byte, error) {
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
	loginsPath := tempPaths[category.Password]
	return retrieveMasterKey(key4Dst, loginsPath)
}

func (p *profile) extractCategory(data *types.BrowserData, cat category.Category, masterKey []byte, path string) {
	var err error
	switch cat {
	case category.Password:
		data.Passwords, err = extractPasswords(masterKey, path)
	case category.Cookie:
		data.Cookies, err = extractCookies(path)
	case category.History:
		data.Histories, err = extractHistories(path)
	case category.Download:
		data.Downloads, err = extractDownloads(path)
	case category.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case category.Extension:
		data.Extensions, err = extractExtensions(path)
	case category.LocalStorage:
		data.LocalStorage, err = extractLocalStorage(path)
	case category.CreditCard, category.SessionStorage:
		// Firefox does not support CreditCard or SessionStorage extraction.
	}
	if err != nil {
		log.Debugf("extract %s for %s: %v", cat, p.label(), err)
	}
}

func (p *profile) countCategory(cat category.Category, path string) int {
	var count int
	var err error
	switch cat {
	case category.Password:
		count, err = countPasswords(path)
	case category.Cookie:
		count, err = countCookies(path)
	case category.History:
		count, err = countHistories(path)
	case category.Download:
		count, err = countDownloads(path)
	case category.Bookmark:
		count, err = countBookmarks(path)
	case category.Extension:
		count, err = countExtensions(path)
	case category.LocalStorage:
		count, err = countLocalStorage(path)
	case category.CreditCard, category.SessionStorage:
		// Firefox does not support CreditCard or SessionStorage.
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, p.label(), err)
	}
	return count
}
