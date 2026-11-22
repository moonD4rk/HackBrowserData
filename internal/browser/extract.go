package browser

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Source is one file or directory an extraction reads.
type Source struct {
	RelativePath    string   // slash-separated, relative to the profile, or to the user data dir when FromUserDataDir
	Fallbacks       []string // older locations, read only when RelativePath is absent
	IsDir           bool
	FromUserDataDir bool // lives outside the profile, such as Local State, or Safari's files elsewhere under ~/Library
	Shared          bool // the whole dir's, such as Local State, so it never makes an extraction present on its own
	Optional        bool // the extraction still runs without it
}

// RelativePaths lists RelativePath and then its Fallbacks, the order they are tried in.
func (s Source) RelativePaths() []string {
	return append([]string{s.RelativePath}, s.Fallbacks...)
}

// CandidatePaths are the RelativePaths for a profile, relative to the user data dir.
func (s Source) CandidatePaths(profile Profile) []string {
	paths := s.RelativePaths()
	if !s.FromUserDataDir {
		for i, relativePath := range paths {
			paths[i] = path.Join(profile.RelativePath, relativePath)
		}
	}
	return paths
}

// Collector sets every record's Browser and Profile; Fail reports a bad record and the extraction goes on.
type Collector interface {
	Add(row record.Record)
	Fail(err error)
}

// ProfileSnapshot is one profile's share of a captured user data dir; an extraction reads nothing else.
type ProfileSnapshot struct {
	UserDataDir UserDataDir
	Profile     Profile
	dirSnapshot snapshot.Snapshot
}

func NewProfileSnapshot(dir UserDataDir, profile Profile, dirSnapshot snapshot.Snapshot) ProfileSnapshot {
	return ProfileSnapshot{UserDataDir: dir, Profile: profile, dirSnapshot: dirSnapshot}
}

// Path skips a candidate that was never captured but returns the error of one whose copy failed.
func (s ProfileSnapshot) Path(source Source) (string, error) {
	for _, relativePath := range source.CandidatePaths(s.Profile) {
		capturedPath, err := s.dirSnapshot.Path(relativePath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return capturedPath, err
	}
	return "", fmt.Errorf("%s: %w", strings.Join(source.RelativePaths(), " or "), fs.ErrNotExist)
}
