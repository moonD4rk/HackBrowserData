package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Archive returns how many files and directories it captured.
func Archive(ctx context.Context, discovery Discovery, options Options, archivePath string) (int, error) {
	var dirs []browser.UserDataDir
	for _, dir := range discovery.Dirs {
		if dir.Browser.Base == browser.Chromium {
			dirs = append(dirs, dir)
		}
	}
	files := archiveFiles(NewPlan(Discovery{Dirs: dirs}, options), dirs)

	snapshotDir, err := snapshot.Temp()
	if err != nil {
		return 0, err
	}
	defer closeSnapshot(snapshotDir)
	if err := snapshotDir.Capture(ctx, files); err != nil {
		return 0, err
	}
	fileCount := countCaptured(snapshotDir, files)
	if fileCount == 0 {
		return 0, errors.New("no decryption-relevant files found to archive")
	}
	if err := snapshotDir.Zip(archivePath); err != nil {
		return 0, fmt.Errorf("write archive %s: %w", archivePath, err)
	}
	return fileCount, nil
}

func archiveFiles(plan Plan, dirs []browser.UserDataDir) []snapshot.File {
	var files []snapshot.File
	seen := make(map[snapshot.File]bool)
	add := func(file snapshot.File) {
		if !seen[file] {
			seen[file] = true
			files = append(files, file)
		}
	}
	for _, dir := range dirs {
		for _, profile := range dir.Profiles {
			for _, fileSpec := range chromium.ArchiveFileSpecs() {
				if file, ok, err := firstOnDisk(dir, profile, fileSpec); err == nil && ok {
					add(file)
				}
			}
		}
		for _, file := range filesOf(plan, dir.ID) {
			add(file)
		}
	}
	return files
}

// countCaptured counts the files that reached the snapshot, reporting each one whose copy failed.
func countCaptured(snapshotDir *snapshot.Dir, files []snapshot.File) int {
	fileCount := 0
	for _, file := range files {
		if _, err := snapshotDir.Snapshot(file.UserDataDirID).Path(file.RelativePath); err != nil {
			log.Warnf("archive: acquire %s/%s: %v", file.UserDataDirID, file.RelativePath, err)
			continue
		}
		fileCount++
	}
	return fileCount
}
