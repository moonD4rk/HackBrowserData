package chromium

import (
	"context"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
	"github.com/moond4rk/hackbrowserdata/types"
)

// snapshotDirID names the one user data dir in a profile's private snapshot, whose root is the profile dir itself.
const snapshotDirID = "profile"

// profile is one Chromium profile under an installation — the leaf extraction
// unit. It reads its own source files but reuses the installation's master keys.
type profile struct {
	profileDir  string
	browserName string
	kind        types.BrowserKind
	extractors  ibrowser.Extractors
	sourcePaths map[category.Category]resolvedPath
}

func (p *profile) name() string {
	if p.profileDir == "" {
		return ""
	}
	return filepath.Base(p.profileDir)
}

func (p *profile) label() string { return p.browserName + "/" + p.name() }

// extract captures the profile's sources into a private snapshot and runs the Chromium base's extractors on it,
// decrypting with the installation's master keys.
func (p *profile) extract(masterKeys masterkey.Set, categories []category.Category) *types.BrowserData {
	data := &types.BrowserData{}
	d, err := snapshot.Temp()
	if err != nil {
		log.Debugf("new snapshot for %s: %v", p.label(), err)
		return data
	}
	defer func() {
		if err := d.Close(); err != nil {
			log.Debugf("remove snapshot for %s: %v", p.label(), err)
		}
	}()

	ctx := context.Background()
	if err := d.Capture(ctx, p.snapshotFiles(categories)); err != nil {
		log.Debugf("capture %s: %v", p.label(), err)
		return data
	}
	keys := func(context.Context) masterkey.Set { return masterKeys }
	ps := ibrowser.NewProfileSnapshot(d.Snapshot(snapshotDirID), ibrowser.Profile{Name: p.name()}, keys)
	out := &dataCollector{data: data, label: p.label()}
	for _, cat := range categories {
		ex, ok := p.extractors[cat]
		if _, found := p.sourcePaths[cat]; !ok || !found {
			continue
		}
		if err := ex.Extract(ctx, ps, out); err != nil {
			log.Debugf("extract %s for %s: %v", cat, p.label(), err)
		}
	}
	return data
}

// snapshotFiles lists, once each, the first candidate on disk of every profile source the categories read.
func (p *profile) snapshotFiles(categories []category.Category) []snapshot.File {
	var files []snapshot.File
	seen := make(map[string]bool)
	for _, cat := range categories {
		if _, found := p.sourcePaths[cat]; !found {
			continue
		}
		for _, src := range p.extractors[cat].Sources {
			if src.Shared {
				continue
			}
			rel, ok := firstOnDisk(p.profileDir, src)
			if !ok || seen[rel] {
				continue
			}
			seen[rel] = true
			files = append(files, snapshot.File{DirID: snapshotDirID, From: p.profileDir, Rel: rel, IsDir: src.IsDir})
		}
	}
	return files
}

func firstOnDisk(dir string, src ibrowser.Source) (string, bool) {
	for _, rel := range src.Paths {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err == nil && info.IsDir() == src.IsDir {
			return rel, true
		}
	}
	return "", false
}

// dataCollector files the records the Chromium base hands back into today's per-category slices.
type dataCollector struct {
	data  *types.BrowserData
	label string
}

func (c *dataCollector) Add(r record.Record) {
	switch v := r.(type) {
	case *record.Login:
		c.data.Passwords = append(c.data.Passwords, *v)
	case *record.Cookie:
		c.data.Cookies = append(c.data.Cookies, *v)
	case *record.Bookmark:
		c.data.Bookmarks = append(c.data.Bookmarks, *v)
	case *record.History:
		c.data.Histories = append(c.data.Histories, *v)
	case *record.Download:
		c.data.Downloads = append(c.data.Downloads, *v)
	case *record.CreditCard:
		c.data.CreditCards = append(c.data.CreditCards, *v)
	case *record.Extension:
		c.data.Extensions = append(c.data.Extensions, *v)
	case *record.LocalStorage:
		c.data.LocalStorage = append(c.data.LocalStorage, *v)
	case *record.SessionStorage:
		c.data.SessionStorage = append(c.data.SessionStorage, *v)
	}
}

func (c *dataCollector) Fail(err error) { log.Debugf("%s: %v", c.label, err) }

// count counts entries per category without decryption.
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
	if accountDB, ok := p.acquireAccountPasswords(session, categories); ok {
		if count, err := countPasswords(accountDB); err != nil {
			log.Debugf("count account passwords for %s: %v", p.label(), err)
		} else {
			counts[category.Password] += count
		}
	}
	return counts
}

// acquireAccountPasswords copies the account-synced password DB into the session, when the
// profile has one alongside the local DB.
func (p *profile) acquireAccountPasswords(session *filemanager.Session, categories []category.Category) (string, bool) {
	accountPath, ok := p.accountPasswordPath(categories)
	if !ok {
		return "", false
	}
	dst := filepath.Join(session.TempDir(), "password_account")
	if err := session.Acquire(accountPath, dst, false); err != nil {
		log.Debugf("acquire account passwords for %s: %v", p.label(), err)
		return "", false
	}
	return dst, true
}

// accountPasswordPath returns the account-synced password DB when the primary source resolved to
// the local Login Data DB. If only Login Data For Account exists, it is already the primary source.
func (p *profile) accountPasswordPath(categories []category.Category) (string, bool) {
	requested := false
	for _, cat := range categories {
		if cat == category.Password {
			requested = true
			break
		}
	}
	primary, ok := p.sourcePaths[category.Password]
	if !requested || !ok || primary.rel == accountLoginData || p.kind == types.ChromiumYandex {
		return "", false
	}
	path := filepath.Join(p.profileDir, accountLoginData)
	info, err := os.Stat(path)
	return path, err == nil && !info.IsDir()
}

// acquireFiles copies source files to the session temp directory.
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

// countCategory calls the appropriate count function for a category.
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
	case category.CreditCard:
		if p.kind == types.ChromiumYandex {
			count, err = countYandexCreditCards(path)
		} else {
			count, err = countCreditCards(path)
		}
	case category.Extension:
		if p.kind == types.ChromiumOpera {
			count, err = countOperaExtensions(path)
		} else {
			count, err = countExtensions(path)
		}
	case category.LocalStorage:
		count, err = countLocalStorage(path)
	case category.SessionStorage:
		count, err = countSessionStorage(path)
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, p.label(), err)
	}
	return count
}
