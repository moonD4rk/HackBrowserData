package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

type keySourceFunc func(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) masterkey.Retrieval

func (f keySourceFunc) MasterKeys(ctx context.Context, dir browser.UserDataDir, dirSnapshot snapshot.Snapshot) masterkey.Retrieval {
	return f(ctx, dir, dirSnapshot)
}

func keysNeverRetrieved(t *testing.T) keySource {
	return keySourceFunc(func(context.Context, browser.UserDataDir, snapshot.Snapshot) masterkey.Retrieval {
		t.Error("keys were retrieved")
		return masterkey.Retrieval{}
	})
}

// fixtureKeys notes where each dir's snapshot keeps its Local State, which is on disk only while that dir runs.
func fixtureKeys(t *testing.T, copied *[]string) keySource {
	return keySourceFunc(func(_ context.Context, _ browser.UserDataDir, dirSnapshot snapshot.Snapshot) masterkey.Retrieval {
		path, err := dirSnapshot.Path("Local State")
		require.NoError(t, err, "keys are read from the dir's own snapshot")
		*copied = append(*copied, path)
		return masterkey.Retrieval{Keys: browsertest.ChromiumKeys}
	})
}

// writerFunc lets a test act as the run writes one extraction's records, before the next extraction starts.
type writerFunc func(dataCategory category.Category, records []record.Record) error

func (f writerFunc) Write(dataCategory category.Category, records []record.Record) error {
	return f(dataCategory, records)
}

func chromeDir(t *testing.T, profiles ...string) browser.UserDataDir {
	t.Helper()
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: t.TempDir()}
	for _, name := range profiles {
		touch(t, dir.Path, name, "History")
		dir.Profiles = append(dir.Profiles, browser.Profile{Name: name, RelativePath: name})
	}
	return dir
}

// chromeDirWithData holds browsertest's two profiles, whose values are sealed with browsertest.ChromiumKeys.
func chromeDirWithData(t *testing.T, id browser.UserDataDirID) browser.UserDataDir {
	t.Helper()
	dir := browser.UserDataDir{ID: id, Browser: lookup(t, "chrome"), Path: t.TempDir()}
	browsertest.WriteChromiumUserDataDir(t, dir.Path, "chrome")
	for _, profile := range browsertest.ChromiumProfiles {
		dir.Profiles = append(dir.Profiles, browser.Profile{Name: profile.Dir, RelativePath: profile.Dir})
	}
	return dir
}

func run(t *testing.T, keys keySource, categories []category.Category, dirs ...browser.UserDataDir) (Summary, string) {
	t.Helper()
	output := OutputOptions{Dir: t.TempDir()}
	summary, err := runner{keySource: keys}.export(context.Background(), Discovery{Dirs: dirs}, categories, output)
	require.NoError(t, err)
	require.NoError(t, summary.Err)
	return summary, output.Dir
}

// TestRun_ExportsTheRecordsReadBeforeAnError breaks one of Chrome's two login stores; the other's logins still export.
func TestRun_ExportsTheRecordsReadBeforeAnError(t *testing.T) {
	dir := chromeDirWithData(t, "chrome")
	dir.Profiles = dir.Profiles[:1]
	broken := filepath.Join(dir.Path, "Default", "Login Data For Account")
	require.NoError(t, os.WriteFile(broken, []byte("not a database"), 0o600))

	summary, out := run(t, keyFileKeys{"chrome": browsertest.ChromiumKeys}, []category.Category{category.Password}, dir)

	require.Len(t, summary.Results, 1)
	assert.Equal(t, OutcomeError, summary.Results[0].Outcome)
	assert.Equal(t, 2, summary.Results[0].Records)
	data, err := os.ReadFile(filepath.Join(out, "password.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"password": "correct-horse-battery-staple"`)
	assert.Contains(t, string(data), `"browser": "Chrome"`, "the collector sets the browser's display name")
	assert.Contains(t, string(data), `"profile": "Default"`)
}

func TestRun_ReportsEachDirWhatNeverRanFirst(t *testing.T) {
	failed := DirFailure{DirID: "edge", Err: errors.New("permission denied")}
	chrome, second := chromeDirWithData(t, "chrome"), chromeDirWithData(t, "chrome-2")
	for _, dir := range []*browser.UserDataDir{&chrome, &second} {
		dir.Profiles = dir.Profiles[:1]
		require.NoError(t, os.Remove(filepath.Join(dir.Path, "Default", "Bookmarks")))
	}
	categories := []category.Category{category.History, category.Bookmark}
	discovery := Discovery{Dirs: []browser.UserDataDir{chrome, second}, DirFailures: []DirFailure{failed}}

	summary, err := runner{keySource: noKeys{}}.export(context.Background(), discovery, categories, OutputOptions{Dir: t.TempDir()})
	require.NoError(t, err)
	require.NoError(t, summary.Err)
	assert.Equal(t, []DirFailure{failed}, summary.DirFailures, "discovery's failures are reported with the run's")
	var got []string
	for _, result := range summary.Results {
		got = append(got, fmt.Sprintf("%s/%s %s", result.DirID, result.Category, result.Outcome))
	}
	assert.Equal(t, []string{
		"chrome/bookmark absent", "chrome/history ok",
		"chrome-2/bookmark absent", "chrome-2/history ok",
	}, got)
}

func TestRun_KeysAreRetrievedOncePerUserDataDir(t *testing.T) {
	calls := 0
	keys := keySourceFunc(func(_ context.Context, _ browser.UserDataDir, dirSnapshot snapshot.Snapshot) masterkey.Retrieval {
		calls++
		_, err := dirSnapshot.Path("Local State")
		require.NoError(t, err, "keys are read from the dir's own snapshot")
		return masterkey.Retrieval{Keys: browsertest.ChromiumKeys, Err: errors.New("v20: no ABE")}
	})
	categories := []category.Category{category.Cookie, category.Password}

	summary, out := run(t, keys, categories, chromeDirWithData(t, "chrome"))

	assert.Equal(t, 1, calls, "two profiles and two categories share one retrieval")
	require.Len(t, summary.DirFailures, 1, "a retrieval error is reported once, for the dir")
	assert.Equal(t, browser.UserDataDirID("chrome"), summary.DirFailures[0].DirID)
	require.ErrorContains(t, summary.DirFailures[0].Err, "v20: no ABE")
	assert.Len(t, summary.Results, 4)
	data, err := os.ReadFile(filepath.Join(out, "cookie.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"value": "theme=dark"`, "keys retrieved beside an error still decrypt")
}

// TestRun_HistoryNeverRetrievesKeys checks nothing to decrypt means no key retrieval, so no prompt on macOS.
func TestRun_HistoryNeverRetrievesKeys(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Local State")
	touch(t, path, "Default", "History")
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}

	run(t, keysNeverRetrieved(t), []category.Category{category.History, category.Download}, dir)
}

func TestRun_RemovesEachSnapshotBeforeTheNextDir(t *testing.T) {
	var copied []string
	checkingEarlierCopies := writerFunc(func(category.Category, []record.Record) error {
		for _, earlier := range copied[:len(copied)-1] {
			_, err := os.Stat(earlier)
			require.ErrorIs(t, err, fs.ErrNotExist, "an earlier dir's copy is still on disk")
		}
		return nil
	})
	discovery := Discovery{Dirs: []browser.UserDataDir{chromeDirWithData(t, "chrome"), chromeDirWithData(t, "chrome-2")}}

	summary := runner{keySource: fixtureKeys(t, &copied), writer: checkingEarlierCopies}.run(
		context.Background(), discovery, []category.Category{category.Cookie})

	require.NoError(t, summary.Err)
	require.Len(t, copied, 2)
	for _, path := range copied {
		_, err := os.Stat(path)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	}
}

func TestRun_StopEndsTheRunBeforeTheNextExtractionAndRemovesTheCopy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var copied []string
	written := 0
	stopping := writerFunc(func(category.Category, []record.Record) error {
		written++
		cancel()
		return nil
	})
	discovery := Discovery{Dirs: []browser.UserDataDir{chromeDirWithData(t, "chrome"), chromeDirWithData(t, "chrome-2")}}

	summary := runner{keySource: fixtureKeys(t, &copied), writer: stopping}.run(
		ctx, discovery, []category.Category{category.Cookie, category.Password})

	require.ErrorIs(t, summary.Err, context.Canceled)
	require.Len(t, summary.Results, 1, "what ran before the stop is still reported")
	assert.Equal(t, 1, written, "an extraction ran after the stop")
	require.Len(t, copied, 1, "the next user data dir is never captured")
	_, err := os.Stat(copied[0])
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRun_OutputFailureStopsTheRunAndLeavesNoCopies(t *testing.T) {
	var copied []string
	diskFull := errors.New("disk full")
	written := 0
	failing := writerFunc(func(category.Category, []record.Record) error {
		written++
		return diskFull
	})
	discovery := Discovery{Dirs: []browser.UserDataDir{chromeDirWithData(t, "chrome"), chromeDirWithData(t, "chrome-2")}}

	summary := runner{keySource: fixtureKeys(t, &copied), writer: failing}.run(
		context.Background(), discovery, []category.Category{category.Cookie, category.Password})

	require.ErrorIs(t, summary.Err, diskFull)
	assert.Equal(t, 1, written, "an extraction ran after the output failed")
	require.Len(t, copied, 1, "the next user data dir is never captured")
	_, err := os.Stat(copied[0])
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestDump_FirefoxPrimaryPasswordOpensEveryProtectedProfile(t *testing.T) {
	home := t.TempDir()
	firefoxBrowser, ok := browser.Lookup("firefox")
	require.True(t, ok)
	profilesDir := filepath.Join(home, filepath.FromSlash(firefoxBrowser.Platform().UserDataDirPaths[0]))
	globalSalt, masterKey := bytes.Repeat([]byte("16-byte-salt-abc"), 3), []byte("firefox-pw-master-key-24")
	for _, profile := range []string{"a.default", "b.work"} {
		browsertest.WriteFirefoxPrimaryPasswordProfile(t, filepath.Join(profilesDir, profile), globalSalt, []byte("hunter2"), [][]byte{masterKey})
	}

	_, summary, err := Dump(context.Background(), DumpOptions{
		Browsers: []browser.Browser{firefoxBrowser}, HomeDir: home, Categories: []category.Category{category.Password},
		Output: OutputOptions{Dir: t.TempDir()}, FirefoxPrimaryPassword: "hunter2",
	})
	require.NoError(t, err)
	require.NoError(t, summary.Err)
	require.Len(t, summary.Results, 2)
	for _, result := range summary.Results {
		assert.Equal(t, OutcomeOK, result.Outcome, result.Profile.Name)
	}
}
