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

// planExtractions decides from which files exist, before anything is copied. It returns the extractions to run in
// output order, the files they read, each listed once, and a Result for each requested extraction that cannot run.
func planExtractions(
	dir browser.UserDataDir, categories []category.Category,
) (extractions []Extraction, files []snapshot.File, skipped []Result) {
	added := make(map[snapshot.File]bool)
	for _, profile := range dir.Profiles {
		for _, dataCategory := range categories {
			extraction := Extraction{DirID: dir.ID, Profile: profile, Category: dataCategory}
			sources, supported := lookupSources(dir, profile, dataCategory)
			if !supported {
				skipped = append(skipped, Result{Extraction: extraction, Outcome: OutcomeUnsupported})
				continue
			}
			located, present, err := locate(dir, profile, sources)
			switch {
			case err != nil:
				skipped = append(skipped, Result{Extraction: extraction, Outcome: OutcomeError, Err: err})
			case !present:
				skipped = append(skipped, Result{Extraction: extraction, Outcome: OutcomeAbsent})
			default:
				extractions = append(extractions, extraction)
				files = appendNew(files, added, located...)
			}
		}
	}
	return extractions, files, skipped
}

func appendNew(files []snapshot.File, added map[snapshot.File]bool, more ...snapshot.File) []snapshot.File {
	for _, file := range more {
		if !added[file] {
			added[file] = true
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
		file, ok, err := firstOnDisk(dir, source.DirRelativePaths(profile), source.IsDir)
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
