// Package snapshot holds the point-in-time copies of browser files that extractors read instead of a live profile.
package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/zipfile"
)

type File struct {
	UserDataDirID   string // the user data dir's ID; also the snapshot it lands in
	UserDataDirPath string // absolute path of the user data dir it is copied from
	RelativePath    string // slash-separated path relative to UserDataDirPath, and to the snapshot
	IsDir           bool
}

// Root is the snapshot directory, laid out as <root>/<UserDataDirID>/<RelativePath>: one Snapshot per user data dir.
type Root struct {
	path   string
	temp   bool
	failed map[string]error // keyed by the cleaned <UserDataDirID>/<RelativePath>, so any spelling of a file finds its failure
}

// Temp creates a private temporary snapshot directory, which Close removes.
func Temp() (*Root, error) {
	path, err := os.MkdirTemp("", "hbd-snapshot-*")
	if err != nil {
		return nil, fmt.Errorf("create snapshot dir: %w", err)
	}
	return &Root{path: path, temp: true, failed: make(map[string]error)}, nil
}

// Open uses an existing snapshot directory in place; Close leaves it there.
func Open(path string) *Root {
	return &Root{path: path, failed: make(map[string]error)}
}

// Unzip extracts an archive into a temporary snapshot directory, rejecting entries that would land outside it.
func Unzip(archivePath string) (*Root, error) {
	root, err := Temp()
	if err != nil {
		return nil, err
	}
	if err := zipfile.Extract(archivePath, root.path); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return root, nil
}

func (r *Root) Path() string {
	return r.path
}

// Capture keeps a file's own failure for Snapshot.Path instead of stopping the rest of the capture.
func (r *Root) Capture(ctx context.Context, files []File) error {
	if err := os.MkdirAll(r.path, 0o700); err != nil {
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
		if err := r.capture(local, file); err != nil {
			r.failed[local] = err
		} else {
			delete(r.failed, local)
		}
	}
	return nil
}

func (r *Root) capture(local string, file File) error {
	target := filepath.Join(r.path, local)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	return copyPath(filepath.Join(file.UserDataDirPath, filepath.FromSlash(file.RelativePath)), target, file.IsDir)
}

func (r *Root) Snapshot(userDataDirID string) Snapshot {
	return Snapshot{userDataDirID: userDataDirID, rootPath: r.path, failed: r.failed}
}

func (r *Root) Close() error {
	if !r.temp {
		return nil
	}
	if err := os.RemoveAll(r.path); err != nil {
		return fmt.Errorf("remove snapshot dir %s: %w", r.path, err)
	}
	return nil
}

type Snapshot struct {
	userDataDirID string
	rootPath      string
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
	fullPath := filepath.Join(s.rootPath, local)
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
