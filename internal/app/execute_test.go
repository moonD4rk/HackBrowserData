package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

var history = browser.Source{Paths: []string{"History"}}

type extractFunc func(ctx context.Context, ps browser.ProfileSnapshot, out browser.Collector) error

// fakeBase reads History for every category, so a test needs one fixture file per profile.
func fakeBase(fns map[category.Category]extractFunc) extractorsFunc {
	return func(browser.UserDataDir, browser.Profile) browser.Extractors {
		ex := make(browser.Extractors, len(fns))
		for c, fn := range fns {
			ex[c] = browser.Extractor{Sources: []browser.Source{history}, Extract: fn}
		}
		return ex
	}
}

func noKeys(t *testing.T) keySource {
	return func(context.Context, browser.UserDataDir, snapshot.Snapshot) (masterkey.Set, error) {
		t.Error("keys were retrieved")
		return nil, nil
	}
}

func chromeDir(t *testing.T, id browser.DirID, profiles ...string) browser.UserDataDir {
	t.Helper()
	dir := browser.UserDataDir{ID: id, Def: lookup(t, "chrome"), Path: t.TempDir()}
	for _, name := range profiles {
		touch(t, dir.Path, name, "History")
		dir.Profiles = append(dir.Profiles, browser.Profile{Name: name, Dir: name})
	}
	return dir
}

func run(t *testing.T, base extractorsFunc, keys keySource, categories []category.Category, dirs ...browser.UserDataDir) (Summary, string) {
	t.Helper()
	req := Request{Categories: categories, Dir: t.TempDir()}
	summary, err := execute(context.Background(), newPlan(Discovery{Dirs: dirs}, req, base), req, base, keys)
	require.NoError(t, err)
	return summary, req.Dir
}

func outcomes(s Summary) map[category.Category]Result {
	out := make(map[category.Category]Result)
	for _, r := range s.Results {
		out[r.Category] = r
	}
	return out
}

func TestExecute_Outcomes(t *testing.T) {
	cookie := func(name string) *record.Cookie { return &record.Cookie{Name: name} }
	base := fakeBase(map[category.Category]extractFunc{
		category.Cookie: func(_ context.Context, _ browser.ProfileSnapshot, out browser.Collector) error {
			out.Add(cookie("a"))
			out.Add(cookie("b"))
			out.Fail(errors.New("value not decrypted"))
			return nil
		},
		category.Password: func(_ context.Context, _ browser.ProfileSnapshot, out browser.Collector) error {
			out.Fail(errors.New("password not decrypted"))
			return nil
		},
		category.History: func(context.Context, browser.ProfileSnapshot, browser.Collector) error { return nil },
		category.Bookmark: func(context.Context, browser.ProfileSnapshot, browser.Collector) error {
			return errors.New("corrupt")
		},
		category.Download: func(context.Context, browser.ProfileSnapshot, browser.Collector) error {
			return fmt.Errorf("History: %w", fs.ErrNotExist)
		},
		category.Extension: func(_ context.Context, _ browser.ProfileSnapshot, out browser.Collector) error {
			out.Add(&record.Extension{Name: "kept"})
			return errors.New("truncated")
		},
	})
	categories := []category.Category{
		category.Cookie, category.Password, category.History, category.Bookmark, category.Download, category.Extension,
	}
	summary, out := run(t, base, noKeys(t), categories, chromeDir(t, "chrome", "Default"))

	got := outcomes(summary)
	assert.Equal(t, OutcomePartial, got[category.Cookie].Outcome)
	assert.Equal(t, 2, got[category.Cookie].Count)
	assert.Equal(t, 1, got[category.Cookie].Failed)
	assert.Equal(t, OutcomeError, got[category.Password].Outcome, "problems and no record")
	require.EqualError(t, got[category.Password].Err, "password not decrypted")
	assert.Equal(t, OutcomeOK, got[category.History].Outcome, "an empty source is ok, not absent")
	assert.Equal(t, OutcomeError, got[category.Bookmark].Outcome)
	assert.Equal(t, OutcomeAbsent, got[category.Download].Outcome, "a source that vanished after planning")
	assert.Equal(t, OutcomeError, got[category.Extension].Outcome)

	data, err := os.ReadFile(filepath.Join(out, "extension.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"kept"`, "records emitted before the error are exported")
	data, err = os.ReadFile(filepath.Join(out, "cookie.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"browser": "Chrome"`, "the collector stamps the browser's display name")
	assert.Contains(t, string(data), `"profile": "Default"`)
}

func TestExecute_KeysAreRetrievedOncePerUserDataDir(t *testing.T) {
	cookie := func(ctx context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
		out.Add(&record.Cookie{Value: string(ps.MasterKeys(ctx)[masterkey.V10])})
		return nil
	}
	login := func(ctx context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
		out.Add(&record.Login{Password: string(ps.MasterKeys(ctx)[masterkey.V10])})
		return nil
	}
	calls := 0
	keys := func(_ context.Context, dir browser.UserDataDir, snap snapshot.Snapshot) (masterkey.Set, error) {
		calls++
		_, err := snap.Path("Default/History")
		require.NoError(t, err, "keys are read from the dir's own snapshot")
		return masterkey.Set{masterkey.V10: []byte(dir.ID)}, errors.New("v20: no ABE")
	}
	base := fakeBase(map[category.Category]extractFunc{category.Cookie: cookie, category.Password: login})
	categories := []category.Category{category.Cookie, category.Password}

	summary, _ := run(t, base, keys, categories, chromeDir(t, "chrome", "Default", "Profile 1"))

	assert.Equal(t, 1, calls)
	last := summary.Results[len(summary.Results)-1]
	assert.Equal(t, Extraction{DirID: "chrome"}, last.Extraction, "a retrieval error is reported once, for the dir")
	assert.Equal(t, OutcomeError, last.Outcome)
	require.ErrorContains(t, last.Err, "v20: no ABE")
	assert.Len(t, summary.Results, 5)
}

// A run that only asks for data without secrets never retrieves a key, so it never prompts on macOS.
func TestExecute_HistoryNeverRetrievesKeys(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Local State")
	touch(t, path, "Default", "History")
	touch(t, path, "Default", "Network", "Cookies")
	dir := browser.UserDataDir{ID: "chrome", Def: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", Dir: "Default"}}}

	run(t, extractorsWith(nil), noKeys(t), []category.Category{category.History, category.Download}, dir)

	calls := 0
	counting := func(context.Context, browser.UserDataDir, snapshot.Snapshot) (masterkey.Set, error) {
		calls++
		return nil, nil
	}
	run(t, extractorsWith(nil), counting, []category.Category{category.Cookie, category.History}, dir)
	assert.Equal(t, 1, calls)
}

// At most one user data dir's copies are on disk: a dir's snapshot is removed before the next dir is captured.
func TestExecute_RemovesEachSnapshotBeforeTheNextDir(t *testing.T) {
	var captured []string
	base := fakeBase(map[category.Category]extractFunc{
		category.History: func(_ context.Context, ps browser.ProfileSnapshot, _ browser.Collector) error {
			for _, earlier := range captured {
				_, err := os.Stat(earlier)
				require.ErrorIs(t, err, fs.ErrNotExist, "an earlier dir's copy is still on disk")
			}
			path, err := ps.Path(history)
			require.NoError(t, err)
			captured = append(captured, path)
			return nil
		},
	})
	run(t, base, noKeys(t), []category.Category{category.History}, chromeDir(t, "chrome", "Default"), chromeDir(t, "chrome-2", "Default"))

	require.Len(t, captured, 2)
	for _, path := range captured {
		_, err := os.Stat(path)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	}
}

func TestExecute_OutputFailureStopsTheRunAndLeavesNoCopies(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny writes here")
	}
	var copied string
	base := fakeBase(map[category.Category]extractFunc{
		category.History: func(_ context.Context, ps browser.ProfileSnapshot, out browser.Collector) error {
			copied, _ = ps.Path(history)
			out.Add(&record.History{URL: "https://example.com"})
			return nil
		},
	})
	req := Request{Categories: []category.Category{category.History}, Dir: t.TempDir()}
	require.NoError(t, os.Chmod(req.Dir, 0o500))
	defer func() { require.NoError(t, os.Chmod(req.Dir, 0o700)) }()

	plan := newPlan(Discovery{Dirs: []browser.UserDataDir{chromeDir(t, "chrome", "Default")}}, req, base)
	_, err := execute(context.Background(), plan, req, base, noKeys(t))
	require.Error(t, err)
	require.NotEmpty(t, copied)
	_, statErr := os.Stat(copied)
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

func TestExecute_DumpKeys(t *testing.T) {
	want := masterkey.Set{masterkey.V10: []byte("dumped")}
	keys := dumpKeys(map[browser.DirID]masterkey.Set{"chrome": want})
	got, err := keys(context.Background(), browser.UserDataDir{ID: "chrome"}, snapshot.Snapshot{})
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = keys(context.Background(), browser.UserDataDir{ID: "edge"}, snapshot.Snapshot{})
	require.NoError(t, err)
	assert.Empty(t, got, "a dump without the dir's keys decrypts nothing")
}
