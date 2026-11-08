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

// Extraction is plain data, so a Result prints as it is and list can key record counts by it.
type Extraction struct {
	UserDataDirID browser.UserDataDirID
	ProfileName   string // Profile.Name
	Category      category.Category
}

// Plan holds no extractor, so it prints and compares as plain data; Execute builds them again with the run's keychain.
type Plan struct {
	Dirs        []DirPlan    // only dirs with something to run, in output order
	Skipped     []Result     // requested extractions that cannot run
	DirFailures []DirFailure // dirs discovery could not read
}

// DirPlan captures each file once, however many of its extractions read it, such as Local State.
type DirPlan struct {
	Dir      browser.UserDataDir
	Files    []snapshot.File
	Profiles []ProfilePlan // only profiles with something to run
}

type ProfilePlan struct {
	Profile    browser.Profile
	Categories []category.Category
}

type extractorsFunc func(browser.UserDataDir, browser.Profile) browser.Extractors

// NewPlan only stats the files the extractors' sources name, so it opens nothing and needs no keychain.
func NewPlan(discovery Discovery, categories []category.Category) Plan {
	return newPlan(discovery, categories, extractorsWith(nil))
}

func newPlan(discovery Discovery, categories []category.Category, extractors extractorsFunc) Plan {
	plan := Plan{DirFailures: append([]DirFailure(nil), discovery.DirFailures...)}
	for _, dir := range discovery.Dirs {
		dirPlan, skipped := planDir(dir, categories, extractors)
		plan.Skipped = append(plan.Skipped, skipped...)
		if len(dirPlan.Profiles) > 0 {
			plan.Dirs = append(plan.Dirs, dirPlan)
		}
	}
	return plan
}

func planDir(dir browser.UserDataDir, categories []category.Category, extractors extractorsFunc) (DirPlan, []Result) {
	dirPlan := DirPlan{Dir: dir}
	var skipped []Result
	captured := make(map[snapshot.File]bool)
	for _, profile := range dir.Profiles {
		profilePlan := ProfilePlan{Profile: profile}
		profileExtractors := extractors(dir, profile)
		for _, dataCategory := range categories {
			extraction := Extraction{UserDataDirID: dir.ID, ProfileName: profile.Name, Category: dataCategory}
			extractor, supported := profileExtractors[dataCategory]
			if !supported {
				skipped = append(skipped, Result{Extraction: extraction, Outcome: OutcomeUnsupported})
				continue
			}
			files, present, err := locate(dir, profile, extractor.Sources)
			switch {
			case err != nil:
				skipped = append(skipped, Result{Extraction: extraction, Outcome: OutcomeError, Err: err})
			case !present:
				skipped = append(skipped, Result{Extraction: extraction, Outcome: OutcomeAbsent})
			default:
				profilePlan.Categories = append(profilePlan.Categories, dataCategory)
				dirPlan.Files = appendNew(dirPlan.Files, captured, files...)
			}
		}
		if len(profilePlan.Categories) > 0 {
			dirPlan.Profiles = append(dirPlan.Profiles, profilePlan)
		}
	}
	return dirPlan, skipped
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
