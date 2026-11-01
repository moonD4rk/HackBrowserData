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

// Source is one file or directory an extractor reads; Paths are its candidates in priority order.
type Source struct {
	Paths    []string // slash-separated, relative to the profile dir, or to the user data dir when Shared
	IsDir    bool
	Shared   bool // belongs to the whole user data dir, such as Local State, so it is captured once per dir
	Optional bool // the extraction still runs without it
}

// Candidates are the Paths for profile p, relative to the user data dir, in priority order.
func (s Source) Candidates(p Profile) []string {
	rels := make([]string, len(s.Paths))
	for i, rel := range s.Paths {
		if !s.Shared {
			rel = path.Join(p.Dir, rel)
		}
		rels[i] = rel
	}
	return rels
}

type Extractor struct {
	Sources []Source
	Extract func(ctx context.Context, ps ProfileSnapshot, out Collector) error
}

type Extractors map[category.Category]Extractor

// Collector stamps every record's Browser and Profile; Fail reports a bad record and the extraction goes on.
type Collector interface {
	Add(r record.Record)
	Fail(err error)
}

type ProfileSnapshot struct {
	Profile    Profile
	snap       snapshot.Snapshot
	masterKeys func(ctx context.Context) masterkey.Set
}

// masterKeys is the user data dir's call-once retrieval, nil for the bases that keep no Chromium keys.
func NewProfileSnapshot(snap snapshot.Snapshot, p Profile, masterKeys func(context.Context) masterkey.Set) ProfileSnapshot {
	return ProfileSnapshot{Profile: p, snap: snap, masterKeys: masterKeys}
}

// Path skips a candidate that was never captured but returns the error of one whose copy failed.
func (ps ProfileSnapshot) Path(src Source) (string, error) {
	for _, rel := range src.Candidates(ps.Profile) {
		p, err := ps.snap.Path(rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return p, err
	}
	return "", fmt.Errorf("%s: %w", strings.Join(src.Paths, " or "), fs.ErrNotExist)
}

// MasterKeys may block on its first call; a key version that could not be retrieved is absent from the Set.
func (ps ProfileSnapshot) MasterKeys(ctx context.Context) masterkey.Set {
	if ps.masterKeys == nil {
		return nil
	}
	return ps.masterKeys(ctx)
}
