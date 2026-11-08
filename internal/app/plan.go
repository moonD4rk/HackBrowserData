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
	UserDataDirID browser.UserDataDirID
	ProfileName   string            // Profile.Name; "" in a result about a whole user data dir
	Category      category.Category // zero in a result about a whole user data dir
}

// Plan lists extractions in output order, each file to capture once, and the requested extractions that cannot run.
type Plan struct {
	Dirs        []browser.UserDataDir
	Files       []snapshot.File
	Extractions []Extraction
	Skipped     []Result
}

type extractorsFunc func(browser.UserDataDir, browser.Profile) browser.Extractors

// NewPlan only stats the files the extractors' sources name, so it opens nothing and needs no keychain.
func NewPlan(discovery Discovery, options Options) Plan {
	return newPlan(discovery, options, extractorsWith(nil))
}

func newPlan(discovery Discovery, options Options, extractors extractorsFunc) Plan {
	plan := Plan{Dirs: discovery.Dirs, Skipped: append([]Result(nil), discovery.Errors...)}
	captured := make(map[snapshot.File]bool)
	for _, dir := range discovery.Dirs {
		for _, profile := range dir.Profiles {
			profileExtractors := extractors(dir, profile)
			for _, dataCategory := range options.Categories {
				extraction := Extraction{UserDataDirID: dir.ID, ProfileName: profile.Name, Category: dataCategory}
				extractor, ok := profileExtractors[dataCategory]
				if !ok {
					plan.Skipped = append(plan.Skipped, Result{Extraction: extraction, Outcome: OutcomeUnsupported})
					continue
				}
				files, ok, err := locate(dir, profile, extractor.Sources)
				if err != nil {
					plan.Skipped = append(plan.Skipped, Result{Extraction: extraction, Outcome: OutcomeError, Err: err})
					continue
				}
				if !ok {
					plan.Skipped = append(plan.Skipped, Result{Extraction: extraction, Outcome: OutcomeAbsent})
					continue
				}
				plan.Extractions = append(plan.Extractions, extraction)
				for _, file := range files {
					if !captured[file] {
						captured[file] = true
						plan.Files = append(plan.Files, file)
					}
				}
			}
		}
	}
	return plan
}

// locate needs every required file and one of the profile's own: Local State alone never makes an extraction present.
// One that reads no file always runs, and a candidate that cannot be checked is an error, not absence.
func locate(dir browser.UserDataDir, profile browser.Profile, sources []browser.Source) ([]snapshot.File, bool, error) {
	if len(sources) == 0 {
		return nil, true, nil
	}
	var files []snapshot.File
	present := false
	for _, source := range sources {
		file, ok, err := firstOnDisk(dir, profile, source)
		switch {
		case err != nil:
			return nil, false, err
		case ok:
			files = append(files, file)
			present = present || !source.Shared
		case !source.Optional:
			return nil, false, nil
		}
	}
	return files, present, nil
}

func firstOnDisk(dir browser.UserDataDir, profile browser.Profile, source browser.Source) (snapshot.File, bool, error) {
	for _, relativePath := range source.CandidatePaths(profile) {
		info, err := os.Stat(filepath.Join(dir.Path, filepath.FromSlash(relativePath)))
		switch {
		case err == nil && info.IsDir() == source.IsDir:
			return snapshot.File{
				UserDataDirID: string(dir.ID), UserDataDirPath: dir.Path, RelativePath: relativePath, IsDir: source.IsDir,
			}, true, nil
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return snapshot.File{}, false, err
		}
	}
	return snapshot.File{}, false, nil
}
