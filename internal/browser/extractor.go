package browser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/keychain"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Source is one file or directory an extractor reads.
type Source struct {
	Path      string   // slash-separated, relative to the profile dir, or to the user data dir when Shared
	Fallbacks []string // older locations, read only when Path is absent
	IsDir     bool
	Shared    bool // belongs to the whole user data dir, such as Local State, so it is captured once per dir
	Optional  bool // the extraction still runs without it
}

// Paths lists Path and then its Fallbacks, the order they are tried in.
func (s Source) Paths() []string {
	return append([]string{s.Path}, s.Fallbacks...)
}

// CandidatePaths are the Paths for a profile, relative to the user data dir.
func (s Source) CandidatePaths(profile Profile) []string {
	paths := s.Paths()
	if !s.Shared {
		for i, relativePath := range paths {
			paths[i] = path.Join(profile.RelativePath, relativePath)
		}
	}
	return paths
}

type Extractor struct {
	Sources []Source
	Extract func(ctx context.Context, input Input, out Collector) error
}

type Extractors map[category.Category]Extractor

// Collector sets every record's Browser and Profile; Fail reports a bad record and the extraction goes on.
type Collector interface {
	Add(row record.Record)
	Fail(err error)
}

// LoginKeychain is what an extractor reads from the run's macOS login keychain.
type LoginKeychain interface {
	InternetPasswords(ctx context.Context) ([]keychain.InternetPassword, error)
}

// Input is what an extractor reads for one profile: its captured files, the dir's master keys, and the run's keychain.
type Input struct {
	Profile       Profile
	snapshot      snapshot.Snapshot
	masterKeys    func(ctx context.Context) masterkey.Set
	loginKeychain LoginKeychain
}

// NewInput takes masterKeys, the dir's call-once retrieval, nil for bases that keep no Chromium keys, and the run's
// login keychain, nil off macOS and in a restore.
func NewInput(
	dirSnapshot snapshot.Snapshot, profile Profile, masterKeys func(context.Context) masterkey.Set, loginKeychain LoginKeychain,
) Input {
	return Input{Profile: profile, snapshot: dirSnapshot, masterKeys: masterKeys, loginKeychain: loginKeychain}
}

// Path skips a candidate that was never captured but returns the error of one whose copy failed.
func (in Input) Path(source Source) (string, error) {
	for _, relativePath := range source.CandidatePaths(in.Profile) {
		capturedPath, err := in.snapshot.Path(relativePath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return capturedPath, err
	}
	return "", fmt.Errorf("%s: %w", strings.Join(source.Paths(), " or "), fs.ErrNotExist)
}

// MasterKeys may block on its first call; a key version that could not be retrieved is absent from the Set.
func (in Input) MasterKeys(ctx context.Context) masterkey.Set {
	if in.masterKeys == nil {
		return nil
	}
	return in.masterKeys(ctx)
}

func (in Input) LoginKeychain() LoginKeychain {
	return in.loginKeychain
}
