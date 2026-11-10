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

var history = browser.Source{Path: "History"}

type extractFunc func(ctx context.Context, input browser.Input, out browser.Collector) error

// fakeBase reads History for every category, so a test needs one fixture file per profile.
func fakeBase(extractFuncs map[category.Category]extractFunc) extractorsFunc {
	return func(browser.UserDataDir, browser.Profile) browser.Extractors {
		extractors := make(browser.Extractors, len(extractFuncs))
		for dataCategory, extract := range extractFuncs {
			extractors[dataCategory] = browser.Extractor{Sources: []browser.Source{history}, Extract: extract}
		}
		return extractors
	}
}

func noKeys(t *testing.T) dirKeysFunc {
	return func(context.Context, browser.UserDataDir, snapshot.Snapshot) (masterkey.Set, error) {
		t.Error("keys were retrieved")
		return nil, nil
	}
}

func chromeDir(t *testing.T, id browser.UserDataDirID, profiles ...string) browser.UserDataDir {
	t.Helper()
	dir := browser.UserDataDir{ID: id, Browser: lookup(t, "chrome"), Path: t.TempDir()}
	for _, name := range profiles {
		touch(t, dir.Path, name, "History")
		dir.Profiles = append(dir.Profiles, browser.Profile{Name: name, RelativePath: name})
	}
	return dir
}

func run(t *testing.T, base extractorsFunc, keys dirKeysFunc, categories []category.Category, dirs ...browser.UserDataDir) (Summary, string) {
	t.Helper()
	output := Output{Dir: t.TempDir()}
	summary, err := execute(context.Background(), Discovery{Dirs: dirs}, categories, output, runner{extractors: base, retrieveKeys: keys})
	require.NoError(t, err)
	return summary, output.Dir
}

func outcomes(summary Summary) map[category.Category]Result {
	out := make(map[category.Category]Result)
	for _, result := range summary.Results {
		out[result.Category] = result
	}
	return out
}

func TestExecute_Outcomes(t *testing.T) {
	cookie := func(name string) *record.Cookie { return &record.Cookie{Name: name} }
	base := fakeBase(map[category.Category]extractFunc{
		category.Cookie: func(_ context.Context, _ browser.Input, out browser.Collector) error {
			out.Add(cookie("a"))
			out.Add(cookie("b"))
			out.Fail(errors.New("value not decrypted"))
			return nil
		},
		category.Password: func(_ context.Context, _ browser.Input, out browser.Collector) error {
			out.Fail(errors.New("password not decrypted"))
			return nil
		},
		category.History: func(context.Context, browser.Input, browser.Collector) error { return nil },
		category.Bookmark: func(context.Context, browser.Input, browser.Collector) error {
			return errors.New("corrupt")
		},
		category.Download: func(context.Context, browser.Input, browser.Collector) error {
			return fmt.Errorf("History: %w", fs.ErrNotExist)
		},
		category.Extension: func(_ context.Context, _ browser.Input, out browser.Collector) error {
			out.Add(&record.Extension{Name: "kept"})
			return errors.New("truncated")
		},
		category.CreditCard: func(_ context.Context, _ browser.Input, out browser.Collector) error {
			out.Add(&record.CreditCard{Name: "kept"})
			return fmt.Errorf("Web Data: %w", fs.ErrNotExist)
		},
	})
	categories := []category.Category{
		category.Cookie, category.Password, category.History, category.Bookmark, category.Download, category.Extension,
		category.CreditCard,
	}
	summary, out := run(t, base, noKeys(t), categories, chromeDir(t, "chrome", "Default"))

	got := outcomes(summary)
	assert.Equal(t, OutcomePartial, got[category.Cookie].Outcome)
	assert.Equal(t, 2, got[category.Cookie].Records)
	assert.Equal(t, 1, got[category.Cookie].Failures)
	assert.Equal(t, OutcomeError, got[category.Password].Outcome, "problems and no record")
	require.EqualError(t, got[category.Password].Err, "password not decrypted")
	assert.Equal(t, OutcomeOK, got[category.History].Outcome, "an empty file is ok, not absent")
	assert.Equal(t, OutcomeError, got[category.Bookmark].Outcome)
	require.EqualError(t, got[category.Bookmark].Err, "corrupt")
	assert.Equal(t, OutcomeAbsent, got[category.Download].Outcome, "a file that vanished after planning")
	assert.Equal(t, OutcomeError, got[category.Extension].Outcome)
	assert.Equal(t, OutcomeError, got[category.CreditCard].Outcome, "a file gone after records were read is an error, not absent")

	data, err := os.ReadFile(filepath.Join(out, "extension.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"kept"`, "records emitted before the error are exported")
	data, err = os.ReadFile(filepath.Join(out, "cookie.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"browser": "Chrome"`, "the collector sets the browser's display name")
	assert.Contains(t, string(data), `"profile": "Default"`)
}

func TestExecute_ReportsEachDirWhatNeverRanFirst(t *testing.T) {
	failed := DirFailure{UserDataDirID: "edge", Err: errors.New("permission denied")}
	chrome, brave := chromeDir(t, "chrome", "Default"), chromeDir(t, "brave", "Default")
	base := fakeBase(map[category.Category]extractFunc{
		category.History: func(context.Context, browser.Input, browser.Collector) error { return nil },
	})
	categories := []category.Category{category.Password, category.History}
	discovery := Discovery{Dirs: []browser.UserDataDir{chrome, brave}, DirFailures: []DirFailure{failed}}

	summary, err := execute(context.Background(), discovery, categories, Output{Dir: t.TempDir()}, runner{extractors: base, retrieveKeys: noKeys(t)})
	require.NoError(t, err)
	assert.Equal(t, []DirFailure{failed}, summary.DirFailures, "discovery's failures are reported with the run's")
	var got []string
	for _, result := range summary.Results {
		got = append(got, fmt.Sprintf("%s/%s %s", result.UserDataDirID, result.Category, result.Outcome))
	}
	assert.Equal(t, []string{
		"chrome/password unsupported", "chrome/history ok",
		"brave/password unsupported", "brave/history ok",
	}, got)
}

func TestExecute_KeysAreRetrievedOncePerUserDataDir(t *testing.T) {
	cookie := func(ctx context.Context, input browser.Input, out browser.Collector) error {
		out.Add(&record.Cookie{Value: string(input.MasterKeys(ctx)[masterkey.V10])})
		return nil
	}
	login := func(ctx context.Context, input browser.Input, out browser.Collector) error {
		out.Add(&record.Login{Password: string(input.MasterKeys(ctx)[masterkey.V10])})
		return nil
	}
	calls := 0
	keys := func(_ context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) (masterkey.Set, error) {
		calls++
		_, err := dirSnapshot.Path("Default/History")
		require.NoError(t, err, "keys are read from the dir's own snapshot")
		return masterkey.Set{masterkey.V10: []byte(dir.ID)}, errors.New("v20: no ABE")
	}
	base := fakeBase(map[category.Category]extractFunc{category.Cookie: cookie, category.Password: login})
	categories := []category.Category{category.Cookie, category.Password}

	summary, out := run(t, base, keys, categories, chromeDir(t, "chrome", "Default", "Profile 1"))

	assert.Equal(t, 1, calls)
	require.Len(t, summary.DirFailures, 1, "a retrieval error is reported once, for the dir")
	assert.Equal(t, browser.UserDataDirID("chrome"), summary.DirFailures[0].UserDataDirID)
	require.ErrorContains(t, summary.DirFailures[0].Err, "v20: no ABE")
	assert.Len(t, summary.Results, 4)
	data, err := os.ReadFile(filepath.Join(out, "cookie.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"value": "chrome"`, "keys retrieved beside an error still decrypt")
}

// TestExecute_HistoryNeverRetrievesKeys checks nothing to decrypt means no key retrieval, so no prompt on macOS.
func TestExecute_HistoryNeverRetrievesKeys(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Local State")
	touch(t, path, "Default", "History")
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}

	run(t, extractorsFor, noKeys(t), []category.Category{category.History, category.Download}, dir)
}

func TestExecute_RemovesEachSnapshotBeforeTheNextDir(t *testing.T) {
	var captured []string
	base := fakeBase(map[category.Category]extractFunc{
		category.History: func(_ context.Context, input browser.Input, _ browser.Collector) error {
			for _, earlier := range captured {
				_, err := os.Stat(earlier)
				require.ErrorIs(t, err, fs.ErrNotExist, "an earlier dir's copy is still on disk")
			}
			path, err := input.Path(history)
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

func TestExecute_StopEndsTheRunBeforeTheNextExtractionAndRemovesTheCopy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var copied []string
	base := fakeBase(map[category.Category]extractFunc{
		category.History: func(_ context.Context, input browser.Input, _ browser.Collector) error {
			path, err := input.Path(history)
			require.NoError(t, err)
			copied = append(copied, path)
			cancel()
			return nil
		},
		category.Download: func(context.Context, browser.Input, browser.Collector) error {
			t.Error("an extraction ran after the stop")
			return nil
		},
	})
	categories := []category.Category{category.History, category.Download}
	discovery := Discovery{Dirs: []browser.UserDataDir{chromeDir(t, "chrome", "Default"), chromeDir(t, "chrome-2", "Default")}}

	_, err := execute(ctx, discovery, categories, Output{Dir: t.TempDir()}, runner{extractors: base, retrieveKeys: noKeys(t)})
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, copied, 1, "the next user data dir is never captured")
	_, statErr := os.Stat(copied[0])
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

func TestExecute_OutputFailureStopsTheRunAndLeavesNoCopies(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny writes here")
	}
	var copied string
	base := fakeBase(map[category.Category]extractFunc{
		category.History: func(_ context.Context, input browser.Input, out browser.Collector) error {
			var err error
			copied, err = input.Path(history)
			require.NoError(t, err)
			out.Add(&record.History{URL: "https://example.com"})
			return nil
		},
		category.Download: func(context.Context, browser.Input, browser.Collector) error {
			t.Error("an extraction ran after the output failed")
			return nil
		},
	})
	output := Output{Dir: t.TempDir()}
	require.NoError(t, os.Chmod(output.Dir, 0o500))
	defer func() { require.NoError(t, os.Chmod(output.Dir, 0o700)) }()

	discovery := Discovery{Dirs: []browser.UserDataDir{chromeDir(t, "chrome", "Default")}}
	_, err := execute(context.Background(), discovery, []category.Category{category.History, category.Download}, output, runner{extractors: base, retrieveKeys: noKeys(t)})
	require.Error(t, err)
	require.NotEmpty(t, copied)
	_, statErr := os.Stat(copied)
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}
