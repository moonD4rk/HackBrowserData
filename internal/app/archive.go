package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// ArchiveFlags are archive's command-line values as cobra parsed them.
type ArchiveFlags struct {
	Browser    string
	Categories string
}

type ArchiveOptions struct {
	Target     Target
	Categories []category.Category
}

// ValidateArchive takes no keychain password: an archive copies files and reads no secret.
func ValidateArchive(flags ArchiveFlags) (ArchiveOptions, error) {
	browsers, err := parseBrowsers(flags.Browser)
	if err != nil {
		return ArchiveOptions{}, err
	}
	categories, err := parseCategories(flags.Categories)
	if err != nil {
		return ArchiveOptions{}, err
	}
	home, err := homeDir()
	if err != nil {
		return ArchiveOptions{}, err
	}
	return ArchiveOptions{Target: Target{Browsers: browsers, HomeDir: home}, Categories: categories}, nil
}

// Archive returns how many files and directories it captured.
func Archive(ctx context.Context, discovery Discovery, categories []category.Category, archivePath string) (int, error) {
	var dirs []browser.UserDataDir
	for _, dir := range discovery.Dirs {
		if dir.Browser.Base == browser.Chromium {
			dirs = append(dirs, dir)
		}
	}
	files := archiveFiles(dirs, categories)

	root, err := snapshot.Temp()
	if err != nil {
		return 0, err
	}
	defer closeRoot(root)
	if err := root.Capture(ctx, files); err != nil {
		return 0, err
	}
	fileCount := countCaptured(root, files)
	if fileCount == 0 {
		return 0, errors.New("no decryption-relevant files found to archive")
	}
	if err := root.Zip(archivePath); err != nil {
		return 0, fmt.Errorf("write archive %s: %w", archivePath, err)
	}
	return fileCount, nil
}

func archiveFiles(dirs []browser.UserDataDir, categories []category.Category) []snapshot.File {
	var files []snapshot.File
	seen := make(map[snapshot.File]bool)
	for _, dir := range dirs {
		for _, profile := range dir.Profiles {
			for _, source := range chromium.ArchiveSources() {
				if file, ok, err := firstOnDisk(dir, source.CandidatePaths(profile), source.IsDir); err == nil && ok {
					files = appendNew(files, seen, file)
				}
			}
		}
		files = appendNew(files, seen, planDir(dir, categories, extractorsFor).files...)
	}
	return files
}

// countCaptured counts the files that reached the snapshot, reporting each one whose copy failed.
func countCaptured(root *snapshot.Root, files []snapshot.File) int {
	fileCount := 0
	for _, file := range files {
		if _, err := root.Snapshot(file.UserDataDirID).Path(file.RelativePath); err != nil {
			log.Warnf("archive: acquire %s/%s: %v", file.UserDataDirID, file.RelativePath, err)
			continue
		}
		fileCount++
	}
	return fileCount
}
