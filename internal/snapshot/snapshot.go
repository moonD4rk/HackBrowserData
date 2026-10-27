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
	DirID string // the user data dir's ID; also the snapshot it lands in
	From  string // absolute path of the user data dir it is copied from
	Rel   string // slash-separated path relative to From, and to the snapshot
	IsDir bool
}

// Dir is a snapshot directory, laid out as <dir>/<DirID>/<Rel>: one Snapshot per user data dir.
type Dir struct {
	path   string
	temp   bool
	failed map[string]error // keyed by the cleaned <DirID>/<Rel> path, so any spelling of a file finds its failure
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
func Unzip(src string) (*Dir, error) {
	d, err := Temp()
	if err != nil {
		return nil, err
	}
	if err := unzip(src, d.path); err != nil {
		return nil, errors.Join(err, d.Close())
	}
	return d, nil
}

func (d *Dir) Path() string {
	return d.path
}

// Capture keeps a file's own failure for Snapshot.Path instead of stopping the rest of the capture.
func (d *Dir) Capture(ctx context.Context, files []File) error {
	if err := os.MkdirAll(d.path, 0o700); err != nil {
		return fmt.Errorf("snapshot dir: %w", err)
	}
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("capture: %w", err)
		}
		local, err := localPath(f.DirID, f.Rel)
		if err != nil {
			continue // Snapshot.Path reports the same error for this file
		}
		if err := d.capture(local, f); err != nil {
			d.failed[local] = err
		} else {
			delete(d.failed, local)
		}
	}
	return nil
}

func (d *Dir) capture(local string, f File) error {
	dst := filepath.Join(d.path, local)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}
	return Copy(filepath.Join(f.From, filepath.FromSlash(f.Rel)), dst, f.IsDir)
}

func (d *Dir) Snapshot(dirID string) Snapshot {
	return Snapshot{dirID: dirID, root: d.path, failed: d.failed}
}

// Close removes a temporary snapshot directory and leaves an opened one in place.
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
	dirID  string
	root   string
	failed map[string]error
}

// Path returns rel's location in the snapshot, or why it is not there: fs.ErrNotExist when it was never captured
// or had vanished before Capture reached it, else the copy's error.
func (s Snapshot) Path(rel string) (string, error) {
	local, err := localPath(s.dirID, rel)
	if err != nil {
		return "", err
	}
	if err, ok := s.failed[local]; ok {
		return "", err
	}
	p := filepath.Join(s.root, local)
	if _, err := os.Lstat(p); err != nil {
		return "", err
	}
	return p, nil
}

// localPath keeps rel inside its own user data dir's snapshot, not merely inside the snapshot directory.
func localPath(dirID, rel string) (string, error) {
	r := filepath.FromSlash(rel)
	if !filepath.IsLocal(dirID) || dirID == "." || strings.ContainsAny(dirID, `/\`) || (r != "" && !filepath.IsLocal(r)) {
		return "", fmt.Errorf("%q in %q is outside the snapshot directory", rel, dirID)
	}
	return filepath.Join(dirID, r), nil
}
