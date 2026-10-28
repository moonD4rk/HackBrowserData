package safari

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	isafari "github.com/moond4rk/hackbrowserdata/internal/browser/safari"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
	"github.com/moond4rk/hackbrowserdata/types"
)

// snapshotDirID names the one user data dir in a profile's private snapshot, whose root is ~/Library.
const snapshotDirID = "library"

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

// asBase is this profile as the Safari base sees it: ~/Library as the user data dir, the Safari directory as the
// default profile's Dir, and a named profile's own directory under the container as its Dir.
func (p *profile) asBase() (ibrowser.UserDataDir, ibrowser.Profile) {
	library := filepath.Dir(p.ctx.legacyHome)
	def := ibrowser.Profile{Name: "default", Dir: filepath.Base(p.ctx.legacyHome)}
	prof := def
	if !p.ctx.isDefault() {
		prof = ibrowser.Profile{Name: p.ctx.name, Dir: "Containers/com.apple.Safari/Data/Library/Safari/Profiles/" + p.ctx.uuidUpper}
	}
	return ibrowser.UserDataDir{Path: library, Profiles: []ibrowser.Profile{def, prof}}, prof
}

// extract captures the profile's files into a private snapshot of ~/Library and runs the Safari base's extractors on
// it; passwords keep today's direct keychain read until the switch-over.
func (p *profile) extract(categories []category.Category, keychainPassword string) *types.BrowserData {
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

	dir, prof := p.asBase()
	extractors := isafari.Extractors(dir, prof, passwordKeychain(keychainPassword))
	ctx := context.Background()
	if err := d.Capture(ctx, snapshotFiles(dir.Path, prof, extractors, categories)); err != nil {
		log.Debugf("capture %s: %v", p.label(), err)
		return data
	}
	ps := ibrowser.NewProfileSnapshot(d.Snapshot(snapshotDirID), prof, nil)
	out := &dataCollector{data: data, label: p.label()}
	for _, cat := range categories {
		ex, ok := extractors[cat]
		if !ok {
			continue
		}
		if err := ex.Extract(ctx, ps, out); err != nil {
			log.Debugf("extract %s for %s: %v", cat, p.label(), err)
		}
	}
	return data
}

// snapshotFiles lists, once each, the first candidate on disk of every source the categories read.
func snapshotFiles(library string, prof ibrowser.Profile, extractors ibrowser.Extractors, categories []category.Category) []snapshot.File {
	var files []snapshot.File
	seen := make(map[string]bool)
	for _, cat := range categories {
		for _, src := range extractors[cat].Sources {
			rel, ok := firstOnDisk(library, prof, src)
			if !ok || seen[rel] {
				continue
			}
			seen[rel] = true
			files = append(files, snapshot.File{DirID: snapshotDirID, From: library, Rel: rel, IsDir: src.IsDir})
		}
	}
	return files
}

func firstOnDisk(library string, prof ibrowser.Profile, src ibrowser.Source) (string, bool) {
	for _, rel := range src.Paths {
		rel = path.Join(prof.Dir, rel)
		info, err := os.Stat(filepath.Join(library, filepath.FromSlash(rel)))
		if err == nil && info.IsDir() == src.IsDir {
			return rel, true
		}
	}
	return "", false
}

// dataCollector files the records the Safari base hands back into today's per-category slices.
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

// count keeps today's counting queries for history and localStorage. Safari's other counters always counted what
// extraction parses, so they count the base's records.
func (p *profile) count(categories []category.Category, keychainPassword string) map[category.Category]int {
	counts := make(map[category.Category]int)
	var parsed, queried []category.Category
	for _, cat := range categories {
		if cat == category.History || cat == category.LocalStorage {
			queried = append(queried, cat)
		} else {
			parsed = append(parsed, cat)
		}
	}
	if len(parsed) > 0 {
		data := p.extract(parsed, keychainPassword)
		for _, cat := range parsed {
			counts[cat] = recordCount(data, cat)
		}
	}
	if len(queried) == 0 {
		return counts
	}

	session, err := filemanager.NewSession()
	if err != nil {
		log.Debugf("new session for %s: %v", p.label(), err)
		return counts
	}
	defer session.Cleanup()
	for cat, file := range p.acquireFiles(session, queried) {
		counts[cat] = p.countCategory(cat, file)
	}
	return counts
}

func recordCount(data *types.BrowserData, cat category.Category) int {
	switch cat {
	case category.Password:
		return len(data.Passwords)
	case category.Cookie:
		return len(data.Cookies)
	case category.Bookmark:
		return len(data.Bookmarks)
	case category.Download:
		return len(data.Downloads)
	case category.Extension:
		return len(data.Extensions)
	default:
		return 0
	}
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

func (p *profile) countCategory(cat category.Category, file string) int {
	var count int
	var err error
	switch cat {
	case category.History:
		count, err = countHistories(file)
	case category.LocalStorage:
		count, err = countLocalStorage(file)
	default:
		// The other categories are counted from their extracted records.
	}
	if err != nil {
		log.Debugf("count %s for %s: %v", cat, p.label(), err)
	}
	return count
}
