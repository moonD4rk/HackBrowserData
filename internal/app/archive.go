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

// Archive returns how many files and directories it captured.
func Archive(ctx context.Context, discovery Discovery, categories []category.Category, archivePath string) (int, error) {
	var dirs []browser.UserDataDir
	for _, dir := range discovery.Dirs {
		if dir.Browser.Base == browser.Chromium {
			dirs = append(dirs, dir)
		}
	}
	files := archiveFiles(dirs, NewPlan(Discovery{Dirs: dirs}, categories))

	snapshots, err := snapshot.Temp()
	if err != nil {
		return 0, err
	}
	defer closeSnapshot(snapshots)
	if err := snapshots.Capture(ctx, files); err != nil {
		return 0, err
	}
	fileCount := countCaptured(snapshots, files)
	if fileCount == 0 {
		return 0, errors.New("no decryption-relevant files found to archive")
	}
	if err := snapshots.Zip(archivePath); err != nil {
		return 0, fmt.Errorf("write archive %s: %w", archivePath, err)
	}
	return fileCount, nil
}

func archiveFiles(dirs []browser.UserDataDir, plan Plan) []snapshot.File {
	var files []snapshot.File
	seen := make(map[snapshot.File]bool)
	for _, dir := range dirs {
		for _, profile := range dir.Profiles {
			for _, source := range chromium.ArchiveSources() {
				if file, ok, err := firstOnDisk(dir, profile, source); err == nil && ok {
					files = appendNew(files, seen, file)
				}
			}
		}
	}
	for _, dirPlan := range plan.Dirs {
		files = appendNew(files, seen, dirPlan.Files...)
	}
	return files
}

// countCaptured counts the files that reached the snapshot, reporting each one whose copy failed.
func countCaptured(snapshots *snapshot.Dir, files []snapshot.File) int {
	fileCount := 0
	for _, file := range files {
		if _, err := snapshots.Snapshot(file.UserDataDirID).Path(file.RelativePath); err != nil {
			log.Warnf("archive: acquire %s/%s: %v", file.UserDataDirID, file.RelativePath, err)
			continue
		}
		fileCount++
	}
	return fileCount
}
