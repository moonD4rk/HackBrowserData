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

// Archive captures every Chromium user data dir into one snapshot directory, records what it holds in the manifest,
// and zips it to dst. Beside the requested categories' files it keeps Local State and each profile's marker, so a
// reader without the manifest still finds the profiles. It returns how many files and directories were captured.
func Archive(ctx context.Context, d Discovery, req Request, dst string) (int, error) {
	var dirs []browser.UserDataDir
	for _, dir := range d.Dirs {
		if dir.Def.Base == browser.Chromium {
			dirs = append(dirs, dir)
		}
	}
	files := archiveFiles(NewPlan(Discovery{Dirs: dirs}, req), dirs)

	snap, err := snapshot.Temp()
	if err != nil {
		return 0, err
	}
	defer closeSnapshot(snap)
	if err := snap.Capture(ctx, files); err != nil {
		return 0, err
	}
	m, n := manifestOf(snap, dirs, files)
	if n == 0 {
		return 0, errors.New("no decryption-relevant files found to archive")
	}
	if err := snap.WriteManifest(m); err != nil {
		return 0, err
	}
	if err := snap.Zip(dst); err != nil {
		return 0, fmt.Errorf("write archive %s: %w", dst, err)
	}
	return n, nil
}

func archiveFiles(p Plan, dirs []browser.UserDataDir) []snapshot.File {
	var files []snapshot.File
	seen := make(map[snapshot.File]bool)
	add := func(f snapshot.File) {
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	for _, dir := range dirs {
		for _, prof := range dir.Profiles {
			for _, src := range chromium.ArchiveSources(dir) {
				if f, ok := firstOnDisk(dir, prof, src); ok {
					add(f)
				}
			}
		}
		for _, f := range filesOf(p, dir.ID) {
			add(f)
		}
	}
	return files
}

// manifestOf lists each dir's files that reached the snapshot; one whose copy failed is reported and left out.
func manifestOf(snap *snapshot.Dir, dirs []browser.UserDataDir, files []snapshot.File) (snapshot.Manifest, int) {
	m := snapshot.Manifest{Version: snapshot.ManifestVersion, Dirs: []snapshot.ManifestDir{}}
	n := 0
	for _, dir := range dirs {
		md := snapshot.ManifestDir{ID: string(dir.ID), Browser: dir.Def.ID, Base: dir.Def.Base.String()}
		for _, p := range dir.Profiles {
			md.Profiles = append(md.Profiles, snapshot.ManifestProfile{Name: p.Name, Dir: p.Dir})
		}
		captured := snap.Snapshot(string(dir.ID))
		for _, f := range files {
			if f.DirID != string(dir.ID) {
				continue
			}
			if _, err := captured.Path(f.Rel); err != nil {
				log.Warnf("archive: acquire %s/%s: %v", f.DirID, f.Rel, err)
				continue
			}
			md.Files = append(md.Files, f.Rel)
			n++
		}
		if len(md.Files) > 0 {
			m.Dirs = append(m.Dirs, md)
		}
	}
	return m, n
}
