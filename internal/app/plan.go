package app

import (
	"os"
	"path"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Extraction is one category of data from one profile of one user data dir. It is plain data, looked up again when it
// runs, so a Plan prints as it is and compares with reflect.DeepEqual.
type Extraction struct {
	DirID    browser.DirID
	Profile  string            // Profile.Name; "" in a result about a whole user data dir
	Category category.Category // zero in a result about a whole user data dir
}

// Plan is what a run will do: the extractions in output order, the files they need captured once each, and the
// requested extractions that cannot run.
type Plan struct {
	Dirs        []browser.UserDataDir
	Files       []snapshot.File
	Extractions []Extraction
	Skipped     []Result
}

type extractorsFunc func(browser.UserDataDir, browser.Profile) browser.Extractors

// NewPlan crosses every discovered profile with the requested categories. It only stats files; it opens nothing.
// Planning reads only the extractors' sources, so it needs no keychain.
func NewPlan(d Discovery, req Request) Plan {
	return newPlan(d, req, extractorsWith(nil))
}

func newPlan(d Discovery, req Request, extractors extractorsFunc) Plan {
	p := Plan{Dirs: d.Dirs, Skipped: append([]Result(nil), d.Failed...)}
	captured := make(map[snapshot.File]bool)
	for _, dir := range d.Dirs {
		for _, prof := range dir.Profiles {
			ex := extractors(dir, prof)
			for _, c := range req.Categories {
				e := Extraction{DirID: dir.ID, Profile: prof.Name, Category: c}
				x, ok := ex[c]
				if !ok {
					p.Skipped = append(p.Skipped, Result{Extraction: e, Outcome: OutcomeUnsupported})
					continue
				}
				files, ok := locate(dir, prof, x.Sources)
				if !ok {
					p.Skipped = append(p.Skipped, Result{Extraction: e, Outcome: OutcomeAbsent})
					continue
				}
				p.Extractions = append(p.Extractions, e)
				for _, f := range files {
					if !captured[f] {
						captured[f] = true
						p.Files = append(p.Files, f)
					}
				}
			}
		}
	}
	return p
}

// locate finds each source's first candidate on disk. An extraction needs every required source and at least one of
// the profile's own; a shared one such as Local State never makes it present by itself. An extraction that reads no
// file, such as Safari's keychain passwords, always runs.
func locate(dir browser.UserDataDir, p browser.Profile, sources []browser.Source) ([]snapshot.File, bool) {
	if len(sources) == 0 {
		return nil, true
	}
	var files []snapshot.File
	present := false
	for _, src := range sources {
		f, ok := firstOnDisk(dir, p, src)
		switch {
		case ok:
			files = append(files, f)
			present = present || !src.Shared
		case !src.Optional:
			return nil, false
		}
	}
	return files, present
}

func firstOnDisk(dir browser.UserDataDir, p browser.Profile, src browser.Source) (snapshot.File, bool) {
	for _, rel := range src.Paths {
		if !src.Shared {
			rel = path.Join(p.Dir, rel)
		}
		info, err := os.Stat(filepath.Join(dir.Path, filepath.FromSlash(rel)))
		if err == nil && info.IsDir() == src.IsDir {
			return snapshot.File{DirID: string(dir.ID), From: dir.Path, Rel: rel, IsDir: src.IsDir}, true
		}
	}
	return snapshot.File{}, false
}
