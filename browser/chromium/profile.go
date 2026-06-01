package chromium

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

// profile is one Chromium profile under an installation — the leaf extraction
// unit. It reads its own source files but reuses the installation's master keys.
type profile struct {
	profileDir  string
	browserName string
	kind        types.BrowserKind
	extractors  map[types.Category]categoryExtractor
	sourcePaths map[types.Category]resolvedPath
}

func (p *profile) name() string {
	if p.profileDir == "" {
		return ""
	}
	return filepath.Base(p.profileDir)
}

func (p *profile) label() string { return p.browserName + "/" + p.name() }

// extract copies the profile's source files to a temp directory and extracts the
// requested categories, decrypting with the installation's master keys.
func (p *profile) extract(masterKeys masterkey.MasterKeys, categories []types.Category) *types.BrowserData {
	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return &types.BrowserData{}
	}
	defer session.Cleanup()

	tempPaths := p.acquireFiles(session, categories)
	data := &types.BrowserData{}
	for _, cat := range categories {
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		p.extractCategory(data, cat, masterKeys, path)
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

// extractCategory calls the appropriate extract function for a category. A custom
// extractor (registered via extractorsForKind) takes precedence over the switch.
func (p *profile) extractCategory(data *types.BrowserData, cat types.Category, masterKeys masterkey.MasterKeys, path string) {
	if ext, ok := p.extractors[cat]; ok {
		if err := ext.extract(masterKeys, path, data); err != nil {
			log.Debugf("extract %s for %s: %v", cat, p.label(), err)
		}
		return
	}

	var err error
	switch cat {
	case types.Password:
		data.Passwords, err = extractPasswords(masterKeys, path)
	case types.Cookie:
		data.Cookies, err = extractCookies(masterKeys, path)
	case types.History:
		data.Histories, err = extractHistories(path)
	case types.Download:
		data.Downloads, err = extractDownloads(path)
	case types.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case types.CreditCard:
		data.CreditCards, err = extractCreditCards(masterKeys, path)
	case types.Extension:
		data.Extensions, err = extractExtensions(path)
	case types.LocalStorage:
		data.LocalStorage, err = extractLocalStorage(path)
	case types.SessionStorage:
		data.SessionStorage, err = extractSessionStorage(path)
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
	case types.CreditCard:
		if p.kind == types.ChromiumYandex {
			count, err = countYandexCreditCards(path)
		} else {
			count, err = countCreditCards(path)
		}
	case types.Extension:
		if p.kind == types.ChromiumOpera {
			count, err = countOperaExtensions(path)
		} else {
			count, err = countExtensions(path)
		}
	case types.LocalStorage:
		count, err = countLocalStorage(path)
	case types.SessionStorage:
		count, err = countSessionStorage(path)
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, p.label(), err)
	}
	return count
}
