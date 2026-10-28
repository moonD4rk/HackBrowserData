package firefox

import (
	"context"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	ifirefox "github.com/moond4rk/hackbrowserdata/internal/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
	"github.com/moond4rk/hackbrowserdata/types"
)

// snapshotDirID names the one user data dir in a profile's private snapshot, whose root is the profile dir itself.
const snapshotDirID = "profile"

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

// extract captures the profile's sources into a private snapshot and runs the Firefox base's extractors on it.
func (p *profile) extract(categories []category.Category) *types.BrowserData {
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

	extractors := ifirefox.Extractors(ibrowser.UserDataDir{}, ibrowser.Profile{})
	ctx := context.Background()
	if err := d.Capture(ctx, p.snapshotFiles(extractors, categories)); err != nil {
		log.Debugf("capture %s: %v", p.label(), err)
		return data
	}
	ps := ibrowser.NewProfileSnapshot(d.Snapshot(snapshotDirID), ibrowser.Profile{Name: p.name()}, nil)
	out := &dataCollector{data: data, label: p.label()}
	for _, cat := range categories {
		ex, ok := extractors[cat]
		if _, found := p.sourcePaths[cat]; !ok || !found {
			continue
		}
		if err := ex.Extract(ctx, ps, out); err != nil {
			log.Debugf("extract %s for %s: %v", cat, p.label(), err)
		}
	}
	return data
}

// snapshotFiles lists, once each, the first candidate on disk of every source the categories read.
func (p *profile) snapshotFiles(extractors ibrowser.Extractors, categories []category.Category) []snapshot.File {
	var files []snapshot.File
	seen := make(map[string]bool)
	for _, cat := range categories {
		if _, found := p.sourcePaths[cat]; !found {
			continue
		}
		for _, src := range extractors[cat].Sources {
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

// dataCollector files the records the Firefox base hands back into today's per-category slices.
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
	case *record.Extension:
		c.data.Extensions = append(c.data.Extensions, *v)
	case *record.LocalStorage:
		c.data.LocalStorage = append(c.data.LocalStorage, *v)
	}
}

func (c *dataCollector) Fail(err error) { log.Debugf("%s: %v", c.label, err) }

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
