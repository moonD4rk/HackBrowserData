package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Extraction is plain data, looked up again when it runs, so a Plan prints as it is and compares with DeepEqual.
type Extraction struct {
	DirID    browser.DirID
	Profile  string            // Profile.Name; "" in a result about a whole user data dir
	Category category.Category // zero in a result about a whole user data dir
}

// Plan lists extractions in output order, each file to capture once, and the requested extractions that cannot run.
type Plan struct {
	Dirs        []browser.UserDataDir
	Files       []snapshot.File
	Extractions []Extraction
	Skipped     []Result
}

type extractorsFunc func(browser.UserDataDir, browser.Profile) browser.Extractors

// NewPlan only stats files and reads only the extractors' sources, so it opens nothing and needs no keychain.
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
				files, ok, err := locate(dir, prof, x.Sources)
				if err != nil {
					p.Skipped = append(p.Skipped, Result{Extraction: e, Outcome: OutcomeError, Err: err})
					continue
				}
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

// An extraction needs every required source and one of the profile's own: Local State alone never makes it present.
// One that reads no file always runs, and a candidate that cannot be checked is an error, not absence.
func locate(dir browser.UserDataDir, p browser.Profile, sources []browser.Source) ([]snapshot.File, bool, error) {
	if len(sources) == 0 {
		return nil, true, nil
	}
	var files []snapshot.File
	present := false
	for _, src := range sources {
		f, ok, err := firstOnDisk(dir, p, src)
		switch {
		case err != nil:
			return nil, false, err
		case ok:
			files = append(files, f)
			present = present || !src.Shared
		case !src.Optional:
			return nil, false, nil
		}
	}
	return files, present, nil
}

func firstOnDisk(dir browser.UserDataDir, p browser.Profile, src browser.Source) (snapshot.File, bool, error) {
	for _, rel := range src.Candidates(p) {
		info, err := os.Stat(filepath.Join(dir.Path, filepath.FromSlash(rel)))
		switch {
		case err == nil && info.IsDir() == src.IsDir:
			return snapshot.File{DirID: string(dir.ID), From: dir.Path, Rel: rel, IsDir: src.IsDir}, true, nil
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return snapshot.File{}, false, err
		}
	}
	return snapshot.File{}, false, nil
}
