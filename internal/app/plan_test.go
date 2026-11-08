package app

import (
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

func extractionsOf(plan dirPlan) []Extraction {
	var extractions []Extraction
	for _, planned := range plan.extractions {
		extractions = append(extractions, planned.Extraction)
	}
	return extractions
}

func TestPlanDir(t *testing.T) {
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
	categories := []category.Category{category.Password, category.Cookie, category.History, category.Download, category.CreditCard}

	plan := planDir(dir, categories, extractorsWith(nil))

	file := func(relativePath string) snapshot.File {
		return snapshot.File{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: relativePath}
	}
	extraction := func(profile string, dataCategory category.Category) Extraction {
		return Extraction{UserDataDirID: "chrome", ProfileName: profile, Category: dataCategory}
	}
	skipped := func(profile string, dataCategory category.Category, outcome Outcome) Result {
		return Result{Extraction: extraction(profile, dataCategory), Outcome: outcome}
	}
	assert.Equal(t, []Extraction{
		extraction("Default", category.Password), extraction("Default", category.Cookie),
		extraction("Default", category.History), extraction("Default", category.Download),
		extraction("Profile 1", category.History), extraction("Profile 1", category.Download),
	}, extractionsOf(plan))
	assert.Equal(t, []snapshot.File{
		file("Default/Login Data"), file("Local State"), file("Default/Network/Cookies"), file("Default/History"),
		file("Profile 1/History"),
	}, plan.files, "Local State and History are copied once, however many extractions read them")
	assert.Equal(t, []Result{
		skipped("Default", category.CreditCard, OutcomeAbsent),
		// Local State alone does not make a profile's passwords present.
		skipped("Profile 1", category.Password, OutcomeAbsent),
		skipped("Profile 1", category.Cookie, OutcomeAbsent),
		skipped("Profile 1", category.CreditCard, OutcomeAbsent),
	}, plan.skipped)
	assert.Equal(t, dir.Profiles[1], plan.extractions[4].profile)
}

func TestPlanDir_PicksTheFirstCandidateOnDisk(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Default", "Cookies")
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}

	plan := planDir(dir, []category.Category{category.Cookie}, extractorsWith(nil))
	assert.Equal(t, []snapshot.File{{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: "Default/Cookies"}}, plan.files)
}

func TestPlanDir_FlatLayoutProfile(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "History")
	dir := browser.UserDataDir{ID: "opera", Browser: lookup(t, "opera"), Path: path, Profiles: []browser.Profile{{Name: "Opera Stable"}}}

	plan := planDir(dir, []category.Category{category.History}, extractorsWith(nil))
	assert.Equal(t, []snapshot.File{{UserDataDirID: "opera", UserDataDirPath: path, RelativePath: "History"}}, plan.files)
	assert.Equal(t, []Extraction{{UserDataDirID: "opera", ProfileName: "Opera Stable", Category: category.History}}, extractionsOf(plan))
}

func TestPlanDir_UnsupportedCategory(t *testing.T) {
	dir := browser.UserDataDir{ID: "x", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}
	none := func(browser.UserDataDir, browser.Profile) browser.Extractors { return browser.Extractors{} }

	plan := planDir(dir, []category.Category{category.CreditCard}, none)
	assert.Equal(t, []Result{{Extraction: Extraction{UserDataDirID: "x", ProfileName: "Default", Category: category.CreditCard}, Outcome: OutcomeUnsupported}},
		plan.skipped)
	assert.Empty(t, plan.extractions)
}

// TestPlanDir_ExtractorWithoutSourcesRuns covers Safari's passwords: read from the keychain, they are never absent.
func TestPlanDir_ExtractorWithoutSourcesRuns(t *testing.T) {
	dir := browser.UserDataDir{ID: "safari", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "default", RelativePath: "Safari"}}}
	keychainOnly := func(browser.UserDataDir, browser.Profile) browser.Extractors {
		return browser.Extractors{category.Password: {}}
	}

	plan := planDir(dir, []category.Category{category.Password}, keychainOnly)
	assert.Equal(t, []Extraction{{UserDataDirID: "safari", ProfileName: "default", Category: category.Password}}, extractionsOf(plan))
	assert.Empty(t, plan.files)
	assert.Empty(t, plan.skipped)
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

func TestPlanDir_UnreadableFileIsAnError(t *testing.T) {
	dir := chromeDir(t, "chrome", "Default")
	lock(t, filepath.Join(dir.Path, "Default"))
	base := fakeBase(map[category.Category]extractFunc{category.History: nil})

	plan := planDir(dir, []category.Category{category.History}, base)

	require.Len(t, plan.skipped, 1)
	assert.Equal(t, OutcomeError, plan.skipped[0].Outcome)
	require.ErrorIs(t, plan.skipped[0].Err, fs.ErrPermission)
	assert.Empty(t, plan.extractions)
	assert.Empty(t, plan.files)
}
