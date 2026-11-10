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

// dirPlan is what runDir does for one user data dir, decided from which files exist before anything is copied.
type dirPlan struct {
	extractions []plannedExtraction // to run, in output order
	files       []snapshot.File     // what they read, each copied once
	skipped     []Result            // requested extractions that will not run, and why
}

// plannedExtraction keeps the extractor that planning checked, so the run uses that same extractor.
type plannedExtraction struct {
	Extraction
	profile   browser.Profile
	extractor browser.Extractor
}

type extractorsFunc func(browser.UserDataDir, browser.Profile) browser.Extractors

func planDir(dir browser.UserDataDir, categories []category.Category, extractors extractorsFunc) dirPlan {
	var plan dirPlan
	captured := make(map[snapshot.File]bool)
	for _, profile := range dir.Profiles {
		profileExtractors := extractors(dir, profile)
		for _, dataCategory := range categories {
			extraction := Extraction{UserDataDirID: dir.ID, ProfileName: profile.Name, Category: dataCategory}
			extractor, supported := profileExtractors[dataCategory]
			if !supported {
				plan.skipped = append(plan.skipped, Result{Extraction: extraction, Outcome: OutcomeUnsupported})
				continue
			}
			files, present, err := locate(dir, profile, extractor.Sources)
			switch {
			case err != nil:
				plan.skipped = append(plan.skipped, Result{Extraction: extraction, Outcome: OutcomeError, Err: err})
			case !present:
				plan.skipped = append(plan.skipped, Result{Extraction: extraction, Outcome: OutcomeAbsent})
			default:
				plan.extractions = append(plan.extractions, plannedExtraction{Extraction: extraction, profile: profile, extractor: extractor})
				plan.files = appendNew(plan.files, captured, files...)
			}
		}
	}
	return plan
}

func appendNew(files []snapshot.File, seen map[snapshot.File]bool, more ...snapshot.File) []snapshot.File {
	for _, file := range more {
		if !seen[file] {
			seen[file] = true
			files = append(files, file)
		}
	}
	return files
}

// locate needs every required file and one of the profile's own: Local State alone never makes an extraction present.
// One that reads no file always runs, and a candidate that cannot be checked is an error, not absence.
func locate(dir browser.UserDataDir, profile browser.Profile, sources []browser.Source) (files []snapshot.File, present bool, err error) {
	if len(sources) == 0 {
		return nil, true, nil
	}
	for _, source := range sources {
		file, ok, err := firstOnDisk(dir, source.CandidatePaths(profile), source.IsDir)
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

// firstOnDisk takes relativePaths under the dir, in the order a source tries them.
func firstOnDisk(dir browser.UserDataDir, relativePaths []string, isDir bool) (snapshot.File, bool, error) {
	for _, relativePath := range relativePaths {
		info, err := os.Stat(filepath.Join(dir.Path, filepath.FromSlash(relativePath)))
		switch {
		case err == nil && info.IsDir() == isDir:
			return snapshot.File{
				UserDataDirID: string(dir.ID), UserDataDirPath: dir.Path, RelativePath: relativePath, IsDir: isDir,
			}, true, nil
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return snapshot.File{}, false, err
		}
	}
	return snapshot.File{}, false, nil
}
