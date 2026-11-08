package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func touch(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
}

func lookup(t *testing.T, name string) browser.Browser {
	t.Helper()
	namedBrowser, ok := browser.Lookup(name)
	require.True(t, ok, name)
	return namedBrowser
}

func TestNewPlan(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Local State")
	touch(t, path, "Default", "Login Data")
	touch(t, path, "Default", "Network", "Cookies")
	touch(t, path, "Default", "History")
	touch(t, path, "Profile 1", "History")
	dir := browser.UserDataDir{
		ID: "chrome", Browser: lookup(t, "chrome"), Path: path,
		Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}, {Name: "Profile 1", RelativePath: "Profile 1"}},
	}
	failed := DirFailure{UserDataDirID: "edge", Err: errors.New("permission denied")}
	categories := []category.Category{category.Password, category.Cookie, category.History, category.Download, category.CreditCard}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}, DirFailures: []DirFailure{failed}}, categories)

	file := func(relativePath string) snapshot.File {
		return snapshot.File{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: relativePath}
	}
	skipped := func(profile string, dataCategory category.Category, outcome Outcome) Result {
		return Result{Extraction: Extraction{UserDataDirID: "chrome", ProfileName: profile, Category: dataCategory}, Outcome: outcome}
	}
	assert.Equal(t, Plan{
		Dirs: []DirPlan{{
			Dir: dir,
			// Local State and History are captured once, however many extractions read them.
			Files: []snapshot.File{
				file("Default/Login Data"), file("Local State"), file("Default/Network/Cookies"), file("Default/History"),
				file("Profile 1/History"),
			},
			Profiles: []ProfilePlan{
				{Profile: dir.Profiles[0], Categories: []category.Category{category.Password, category.Cookie, category.History, category.Download}},
				{Profile: dir.Profiles[1], Categories: []category.Category{category.History, category.Download}},
			},
		}},
		Skipped: []Result{
			skipped("Default", category.CreditCard, OutcomeAbsent),
			// Local State alone does not make a profile's passwords present.
			skipped("Profile 1", category.Password, OutcomeAbsent),
			skipped("Profile 1", category.Cookie, OutcomeAbsent),
			skipped("Profile 1", category.CreditCard, OutcomeAbsent),
		},
		DirFailures: []DirFailure{failed},
	}, got)
}

func TestNewPlan_PicksTheFirstCandidateOnDisk(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Default", "Cookies")
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, []category.Category{category.Cookie})
	require.Len(t, got.Dirs, 1)
	assert.Equal(t, []snapshot.File{{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: "Default/Cookies"}}, got.Dirs[0].Files)
}

func TestNewPlan_FlatLayoutProfile(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "History")
	dir := browser.UserDataDir{ID: "opera", Browser: lookup(t, "opera"), Path: path, Profiles: []browser.Profile{{Name: "Opera Stable"}}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, []category.Category{category.History})
	require.Len(t, got.Dirs, 1)
	assert.Equal(t, []snapshot.File{{UserDataDirID: "opera", UserDataDirPath: path, RelativePath: "History"}}, got.Dirs[0].Files)
	assert.Equal(t, []ProfilePlan{{Profile: dir.Profiles[0], Categories: []category.Category{category.History}}}, got.Dirs[0].Profiles)
}

func TestNewPlan_UnsupportedCategory(t *testing.T) {
	dir := browser.UserDataDir{ID: "x", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}
	none := func(browser.UserDataDir, browser.Profile) browser.Extractors { return browser.Extractors{} }

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, []category.Category{category.CreditCard}, none)
	assert.Equal(t, []Result{{Extraction: Extraction{UserDataDirID: "x", ProfileName: "Default", Category: category.CreditCard}, Outcome: OutcomeUnsupported}},
		got.Skipped)
	assert.Empty(t, got.Dirs)
}

// TestNewPlan_ExtractorWithoutSourcesRuns covers Safari's passwords: read from the keychain, they are never absent.
func TestNewPlan_ExtractorWithoutSourcesRuns(t *testing.T) {
	dir := browser.UserDataDir{ID: "safari", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "default", RelativePath: "Safari"}}}
	keychainOnly := func(browser.UserDataDir, browser.Profile) browser.Extractors {
		return browser.Extractors{category.Password: {}}
	}

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, []category.Category{category.Password}, keychainOnly)
	assert.Equal(t, []DirPlan{{Dir: dir, Profiles: []ProfilePlan{{Profile: dir.Profiles[0], Categories: []category.Category{category.Password}}}}}, got.Dirs)
	assert.Empty(t, got.Skipped)
}

// lock makes dir unsearchable until the test ends, so a Stat of anything under it fails with permission denied.
func lock(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mode bits do not restrict Stat on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	require.NoError(t, os.Chmod(dir, 0))
	t.Cleanup(func() { assert.NoError(t, os.Chmod(dir, 0o700)) })
}

func TestNewPlan_UnreadableFileIsAnError(t *testing.T) {
	dir := chromeDir(t, "chrome", "Default")
	lock(t, filepath.Join(dir.Path, "Default"))
	base := fakeBase(map[category.Category]extractFunc{category.History: nil})

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, []category.Category{category.History}, base)

	require.Len(t, got.Skipped, 1)
	assert.Equal(t, OutcomeError, got.Skipped[0].Outcome)
	require.ErrorIs(t, got.Skipped[0].Err, fs.ErrPermission)
	assert.Empty(t, got.Dirs)
}
