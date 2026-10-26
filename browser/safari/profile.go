package safari

import (
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// profile is one Safari profile — the leaf extraction unit. Passwords come from
// the shared macOS Keychain; everything else reads from the profile's directories.
type profile struct {
	ctx         profileContext
	browserName string
	sourcePaths map[category.Category]resolvedPath
}

func (p *profile) name() string  { return p.ctx.name }
func (p *profile) label() string { return p.browserName + "/" + p.name() }

func (p *profile) dir() string {
	if p.ctx.isDefault() {
		return p.ctx.legacyHome
	}
	return filepath.Join(p.ctx.container, "Safari", "Profiles", p.ctx.uuidUpper)
}

func (p *profile) extract(categories []category.Category, keychainPassword string) *types.BrowserData {
	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return &types.BrowserData{}
	}
	defer session.Cleanup()

	tempPaths := p.acquireFiles(session, categories)

	data := &types.BrowserData{}
	for _, cat := range categories {
		// Keychain is user-scope, not per-profile — attribute only to default to avoid duplicates.
		if cat == category.Password {
			if p.ctx.isDefault() {
				p.extractCategory(data, cat, "", keychainPassword)
			}
			continue
		}
		// Extension plists (AppExtensions + WebExtensions) live directly in the container
		// and are read in-place; attribute to default only until per-profile layouts are verified.
		if cat == category.Extension {
			if p.ctx.isDefault() {
				p.extractCategory(data, cat, "", keychainPassword)
			}
			continue
		}
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		p.extractCategory(data, cat, path, keychainPassword)
	}
	return data
}

func (p *profile) count(categories []category.Category, keychainPassword string) map[category.Category]int {
	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return nil
	}
	defer session.Cleanup()

	tempPaths := p.acquireFiles(session, categories)

	counts := make(map[category.Category]int)
	for _, cat := range categories {
		if cat == category.Password {
			if p.ctx.isDefault() {
				counts[cat] = p.countCategory(cat, "", keychainPassword)
			}
			continue
		}
		if cat == category.Extension {
			if p.ctx.isDefault() {
				counts[cat] = p.countCategory(cat, "", keychainPassword)
			}
			continue
		}
		path, ok := tempPaths[cat]
		if !ok {
			continue
		}
		counts[cat] = p.countCategory(cat, path, keychainPassword)
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

func (p *profile) extractCategory(data *types.BrowserData, cat category.Category, path, keychainPassword string) {
	var err error
	switch cat {
	case category.Password:
		data.Passwords, err = extractPasswords(keychainPassword)
	case category.History:
		data.Histories, err = extractHistories(path)
	case category.Cookie:
		data.Cookies, err = extractCookies(path)
	case category.Bookmark:
		data.Bookmarks, err = extractBookmarks(path)
	case category.Download:
		data.Downloads, err = extractDownloads(path, p.ctx.downloadOwnerUUID())
	case category.LocalStorage:
		data.LocalStorage, err = extractLocalStorage(path)
	case category.Extension:
		data.Extensions, err = extractExtensions(p.ctx.container)
	default:
		return
	}
	if err != nil {
		log.Debugf("extract %s for %s: %v", cat, p.label(), err)
	}
}

func (p *profile) countCategory(cat category.Category, path, keychainPassword string) int {
	var count int
	var err error
	switch cat {
	case category.Password:
		count, err = countPasswords(keychainPassword)
	case category.History:
		count, err = countHistories(path)
	case category.Cookie:
		count, err = countCookies(path)
	case category.Bookmark:
		count, err = countBookmarks(path)
	case category.Download:
		count, err = countDownloads(path, p.ctx.downloadOwnerUUID())
	case category.LocalStorage:
		count, err = countLocalStorage(path)
	case category.Extension:
		count, err = countExtensions(p.ctx.container)
	default:
		// Unsupported categories silently return 0.
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, p.label(), err)
	}
	return count
}
