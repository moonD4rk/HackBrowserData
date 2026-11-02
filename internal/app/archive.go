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

// Archive also keeps Local State and each profile's marker, so a reader without the manifest still finds the profiles.
// It returns how many files and directories were captured.
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
	manifest, fileCount := manifestOf(snapshotDir, dirs, files)
	if fileCount == 0 {
		return 0, errors.New("no decryption-relevant files found to archive")
	}
	if err := snapshotDir.WriteManifest(manifest); err != nil {
		return 0, err
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
			for _, fileSpec := range chromium.ArchiveFileSpecs(dir) {
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

// manifestOf lists each dir's files that reached the snapshot; one whose copy failed is reported and left out.
func manifestOf(snapshotDir *snapshot.Dir, dirs []browser.UserDataDir, files []snapshot.File) (snapshot.Manifest, int) {
	manifest := snapshot.Manifest{Version: snapshot.ManifestVersion, Dirs: []snapshot.ManifestDir{}}
	fileCount := 0
	for _, dir := range dirs {
		manifestEntry := snapshot.ManifestDir{ID: string(dir.ID), Browser: dir.Browser.Name, Base: dir.Browser.Base.String()}
		for _, profile := range dir.Profiles {
			manifestEntry.Profiles = append(manifestEntry.Profiles, snapshot.ManifestProfile{Name: profile.Name, Dir: profile.Dir})
		}
		captured := snapshotDir.Snapshot(string(dir.ID))
		for _, file := range files {
			if file.UserDataDirID != string(dir.ID) {
				continue
			}
			if _, err := captured.Path(file.RelativePath); err != nil {
				log.Warnf("archive: acquire %s/%s: %v", file.UserDataDirID, file.RelativePath, err)
				continue
			}
			manifestEntry.Files = append(manifestEntry.Files, file.RelativePath)
			fileCount++
		}
		if len(manifestEntry.Files) > 0 {
			manifest.Dirs = append(manifest.Dirs, manifestEntry)
		}
	}
	return manifest, fileCount
}
