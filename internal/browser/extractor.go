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

// Extractor declares the sources a category needs and how to parse them.
type Extractor struct {
	Sources []Source
	Extract func(ctx context.Context, ps ProfileSnapshot, out Collector) error
}

// Extractors maps each category a profile supports to its extractor.
type Extractors map[category.Category]Extractor

// Collector receives one extraction's output and stamps Browser and Profile on every record, so no extractor can omit
// them. Fail records one record-level problem, a secret that could not be recovered or a row that could not be parsed,
// and the extraction goes on.
type Collector interface {
	Add(r record.Record)
	Fail(err error)
}

// ProfileSnapshot is what an extractor is handed: its profile's view of the user data dir's snapshot, and the dir's keys.
type ProfileSnapshot struct {
	Profile    Profile
	snap       snapshot.Snapshot
	masterKeys func(ctx context.Context) masterkey.Set
}

// NewProfileSnapshot gives a profile its share of snap. masterKeys is the user data dir's call-once retrieval, nil for
// the bases that keep no Chromium keys.
func NewProfileSnapshot(snap snapshot.Snapshot, p Profile, masterKeys func(context.Context) masterkey.Set) ProfileSnapshot {
	return ProfileSnapshot{Profile: p, snap: snap, masterKeys: masterKeys}
}

// Path returns where src was captured: its first candidate present in the snapshot. A candidate that was never captured
// is skipped; one whose copy failed returns that copy's error.
func (ps ProfileSnapshot) Path(src Source) (string, error) {
	for _, rel := range src.Paths {
		if !src.Shared {
			rel = path.Join(ps.Profile.Dir, rel)
		}
		p, err := ps.snap.Path(rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return p, err
	}
	return "", fmt.Errorf("%s: %w", strings.Join(src.Paths, " or "), fs.ErrNotExist)
}

// MasterKeys returns the user data dir's Chromium keys, one per version; a version that could not be retrieved is
// absent. It may block on the first call, which retrieves them for every profile of the dir.
func (ps ProfileSnapshot) MasterKeys(ctx context.Context) masterkey.Set {
	if ps.masterKeys == nil {
		return nil
	}
	return ps.masterKeys(ctx)
}
