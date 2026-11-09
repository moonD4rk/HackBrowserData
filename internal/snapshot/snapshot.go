// Package snapshot holds the point-in-time copies of browser files that extractors read instead of a live profile.
package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	UserDataDirID   string // the user data dir's ID; also the snapshot it lands in
	UserDataDirPath string // absolute path of the user data dir it is copied from
	RelativePath    string // slash-separated path relative to UserDataDirPath, and to the snapshot
	IsDir           bool
}

// Dir is a snapshot directory, laid out as <dir>/<UserDataDirID>/<RelativePath>: one Snapshot per user data dir.
type Dir struct {
	path   string
	temp   bool
	failed map[string]error // keyed by the cleaned <UserDataDirID>/<RelativePath>, so any spelling of a file finds its failure
}

// Temp creates a private temporary snapshot directory, which Close removes.
func Temp() (*Dir, error) {
	path, err := os.MkdirTemp("", "hbd-snapshot-*")
	if err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}
	return &Dir{path: path, temp: true, failed: make(map[string]error)}, nil
}

// Open uses an existing snapshot directory in place; Close leaves it there.
func Open(path string) *Dir {
	return &Dir{path: path, failed: make(map[string]error)}
}

// Unzip extracts an archive into a temporary snapshot directory, rejecting entries that would land outside it.
func Unzip(archivePath string) (*Dir, error) {
	snapshotDir, err := Temp()
	if err != nil {
		return nil, err
	}
	if err := unzip(archivePath, snapshotDir.path); err != nil {
		return nil, errors.Join(err, snapshotDir.Close())
	}
	return snapshotDir, nil
}

func (d *Dir) Path() string {
	return d.path
}

// Capture keeps a file's own failure for Snapshot.Path instead of stopping the rest of the capture.
func (d *Dir) Capture(ctx context.Context, files []File) error {
	if err := os.MkdirAll(d.path, 0o700); err != nil {
		return fmt.Errorf("snapshot dir: %w", err)
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("capture: %w", err)
		}
		local, err := localPath(file.UserDataDirID, file.RelativePath)
		if err != nil {
			continue // Snapshot.Path reports the same error for this file
		}
		if err := d.capture(local, file); err != nil {
			d.failed[local] = err
		} else {
			delete(d.failed, local)
		}
	}
	return nil
}

func (d *Dir) capture(local string, file File) error {
	target := filepath.Join(d.path, local)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	return copyPath(filepath.Join(file.UserDataDirPath, filepath.FromSlash(file.RelativePath)), target, file.IsDir)
}

func (d *Dir) Snapshot(userDataDirID string) Snapshot {
	return Snapshot{userDataDirID: userDataDirID, root: d.path, failed: d.failed}
}

func (d *Dir) Close() error {
	if !d.temp {
		return nil
	}
	if err := os.RemoveAll(d.path); err != nil {
		return fmt.Errorf("remove snapshot dir %s: %w", d.path, err)
	}
	return nil
}

type Snapshot struct {
	userDataDirID string
	root          string
	failed        map[string]error
}

// Path reports fs.ErrNotExist for a file never captured or gone before Capture reached it, else the copy's error.
func (s Snapshot) Path(relativePath string) (string, error) {
	local, err := localPath(s.userDataDirID, relativePath)
	if err != nil {
		return "", err
	}
	if err, ok := s.failed[local]; ok {
		return "", err
	}
	fullPath := filepath.Join(s.root, local)
	if _, err := os.Lstat(fullPath); err != nil {
		return "", err
	}
	return fullPath, nil
}

// localPath keeps relativePath inside its own user data dir's snapshot, not merely inside the snapshot directory.
func localPath(userDataDirID, relativePath string) (string, error) {
	nativePath := filepath.FromSlash(relativePath)
	if !filepath.IsLocal(userDataDirID) || userDataDirID == "." || strings.ContainsAny(userDataDirID, `/\`) ||
		(nativePath != "" && !filepath.IsLocal(nativePath)) {
		return "", fmt.Errorf("%q in %q is outside the snapshot directory", relativePath, userDataDirID)
	}
	return filepath.Join(userDataDirID, nativePath), nil
}
