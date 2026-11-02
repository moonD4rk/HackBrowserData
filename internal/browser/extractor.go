package browser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// FileSpec is one file or directory an extractor reads; Paths are its candidates in priority order.
type FileSpec struct {
	Paths    []string // slash-separated, relative to the profile dir, or to the user data dir when Shared
	IsDir    bool
	Shared   bool // belongs to the whole user data dir, such as Local State, so it is captured once per dir
	Optional bool // the extraction still runs without it
}

// CandidatePaths are the Paths for a profile, relative to the user data dir, in priority order.
func (s FileSpec) CandidatePaths(profile Profile) []string {
	paths := make([]string, len(s.Paths))
	for i, relativePath := range s.Paths {
		if !s.Shared {
			relativePath = path.Join(profile.Dir, relativePath)
		}
		paths[i] = relativePath
	}
	return paths
}

type Extractor struct {
	FileSpecs []FileSpec
	Extract   func(ctx context.Context, profileSnapshot ProfileSnapshot, out Collector) error
}

type Extractors map[category.Category]Extractor

// Collector sets every record's Browser and Profile; Fail reports a bad record and the extraction goes on.
type Collector interface {
	Add(row record.Record)
	Fail(err error)
}

type ProfileSnapshot struct {
	Profile    Profile
	snapshot   snapshot.Snapshot
	masterKeys func(ctx context.Context) masterkey.Set
}

// NewProfileSnapshot takes masterKeys, the dir's call-once retrieval, nil for bases that keep no Chromium keys.
func NewProfileSnapshot(dirSnapshot snapshot.Snapshot, profile Profile, masterKeys func(context.Context) masterkey.Set) ProfileSnapshot {
	return ProfileSnapshot{Profile: profile, snapshot: dirSnapshot, masterKeys: masterKeys}
}

// Path skips a candidate that was never captured but returns the error of one whose copy failed.
func (ps ProfileSnapshot) Path(fileSpec FileSpec) (string, error) {
	for _, relativePath := range fileSpec.CandidatePaths(ps.Profile) {
		capturedPath, err := ps.snapshot.Path(relativePath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return capturedPath, err
	}
	return "", fmt.Errorf("%s: %w", strings.Join(fileSpec.Paths, " or "), fs.ErrNotExist)
}

// MasterKeys may block on its first call; a key version that could not be retrieved is absent from the Set.
func (ps ProfileSnapshot) MasterKeys(ctx context.Context) masterkey.Set {
	if ps.masterKeys == nil {
		return nil
	}
	return ps.masterKeys(ctx)
}
