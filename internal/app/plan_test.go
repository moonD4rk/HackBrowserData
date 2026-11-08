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
	failed := dirFailure("edge", errors.New("permission denied"))
	options := Options{Categories: []category.Category{category.Password, category.Cookie, category.History, category.Download, category.CreditCard}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}, Errors: []Result{failed}}, options)

	file := func(relativePath string) snapshot.File {
		return snapshot.File{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: relativePath}
	}
	extraction := func(profile string, dataCategory category.Category) Extraction {
		return Extraction{UserDataDirID: "chrome", ProfileName: profile, Category: dataCategory}
	}
	skipped := func(profile string, dataCategory category.Category, outcome Outcome) Result {
		return Result{Extraction: extraction(profile, dataCategory), Outcome: outcome}
	}
	assert.Equal(t, Plan{
		Dirs: []browser.UserDataDir{dir},
		// Local State and History are captured once, however many extractions read them.
		Files: []snapshot.File{
			file("Default/Login Data"), file("Local State"), file("Default/Network/Cookies"), file("Default/History"),
			file("Profile 1/History"),
		},
		Extractions: []Extraction{
			extraction("Default", category.Password), extraction("Default", category.Cookie),
			extraction("Default", category.History), extraction("Default", category.Download),
			extraction("Profile 1", category.History), extraction("Profile 1", category.Download),
		},
		Skipped: []Result{
			failed,
			skipped("Default", category.CreditCard, OutcomeAbsent),
			// Local State alone does not make a profile's passwords present.
			skipped("Profile 1", category.Password, OutcomeAbsent),
			skipped("Profile 1", category.Cookie, OutcomeAbsent),
			skipped("Profile 1", category.CreditCard, OutcomeAbsent),
		},
	}, got)
}

func TestNewPlan_PicksTheFirstCandidateOnDisk(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Default", "Cookies")
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Options{Categories: []category.Category{category.Cookie}})
	assert.Equal(t, []snapshot.File{{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: "Default/Cookies"}}, got.Files)
}

func TestNewPlan_FlatLayoutProfile(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "History")
	dir := browser.UserDataDir{ID: "opera", Browser: lookup(t, "opera"), Path: path, Profiles: []browser.Profile{{Name: "Opera Stable"}}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Options{Categories: []category.Category{category.History}})
	assert.Equal(t, []snapshot.File{{UserDataDirID: "opera", UserDataDirPath: path, RelativePath: "History"}}, got.Files)
	assert.Equal(t, []Extraction{{UserDataDirID: "opera", ProfileName: "Opera Stable", Category: category.History}}, got.Extractions)
}

func TestNewPlan_UnsupportedCategory(t *testing.T) {
	dir := browser.UserDataDir{ID: "x", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}
	none := func(browser.UserDataDir, browser.Profile) browser.Extractors { return browser.Extractors{} }

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Options{Categories: []category.Category{category.CreditCard}}, none)
	assert.Equal(t, []Result{{Extraction: Extraction{UserDataDirID: "x", ProfileName: "Default", Category: category.CreditCard}, Outcome: OutcomeUnsupported}},
		got.Skipped)
	assert.Empty(t, got.Extractions)
}

// TestNewPlan_ExtractorWithoutFileSpecsRuns covers Safari's passwords: read from the keychain, they are never absent.
func TestNewPlan_ExtractorWithoutFileSpecsRuns(t *testing.T) {
	dir := browser.UserDataDir{ID: "safari", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "default", RelativePath: "Safari"}}}
	keychainOnly := func(browser.UserDataDir, browser.Profile) browser.Extractors {
		return browser.Extractors{category.Password: {}}
	}

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Options{Categories: []category.Category{category.Password}}, keychainOnly)
	assert.Equal(t, []Extraction{{UserDataDirID: "safari", ProfileName: "default", Category: category.Password}}, got.Extractions)
	assert.Empty(t, got.Files)
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

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Options{Categories: []category.Category{category.History}}, base)

	require.Len(t, got.Skipped, 1)
	assert.Equal(t, OutcomeError, got.Skipped[0].Outcome)
	require.ErrorIs(t, got.Skipped[0].Err, fs.ErrPermission)
	assert.Empty(t, got.Extractions)
	assert.Empty(t, got.Files)
}
