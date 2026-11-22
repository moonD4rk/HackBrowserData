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

func TestPlanExtractions(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Local State")
	touch(t, path, "Default", "Login Data")
	touch(t, path, "Default", "Network", "Cookies")
	touch(t, path, "Default", "History")
	touch(t, path, "Profile 1", "History")
	first := browser.Profile{Name: "Default", RelativePath: "Default"}
	second := browser.Profile{Name: "Profile 1", RelativePath: "Profile 1"}
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{first, second}}
	categories := []category.Category{category.Password, category.Cookie, category.History, category.Download, category.CreditCard}

	extractions, files, skipped := planExtractions(dir, categories)

	file := func(relativePath string) snapshot.File {
		return snapshot.File{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: relativePath}
	}
	extraction := func(profile browser.Profile, dataCategory category.Category) Extraction {
		return Extraction{DirID: "chrome", Profile: profile, Category: dataCategory}
	}
	absent := func(profile browser.Profile, dataCategory category.Category) Result {
		return Result{Extraction: extraction(profile, dataCategory), Outcome: OutcomeAbsent}
	}
	assert.Equal(t, []Extraction{
		extraction(first, category.Password), extraction(first, category.Cookie),
		extraction(first, category.History), extraction(first, category.Download),
		extraction(second, category.History), extraction(second, category.Download),
	}, extractions)
	assert.Equal(t, []snapshot.File{
		file("Default/Login Data"), file("Local State"), file("Default/Network/Cookies"), file("Default/History"),
		file("Profile 1/History"),
	}, files, "Local State and History are copied once, however many extractions read them")
	assert.Equal(t, []Result{
		absent(first, category.CreditCard),
		// Local State alone does not make a profile's passwords present.
		absent(second, category.Password),
		absent(second, category.Cookie),
		absent(second, category.CreditCard),
	}, skipped)
}

func TestPlanExtractions_PicksTheFirstCandidateOnDisk(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Default", "Cookies")
	dir := browser.UserDataDir{ID: "chrome", Browser: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", RelativePath: "Default"}}}

	_, files, _ := planExtractions(dir, []category.Category{category.Cookie})
	assert.Equal(t, []snapshot.File{{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: "Default/Cookies"}}, files)

	touch(t, path, "Default", "Network", "Cookies")
	_, files, _ = planExtractions(dir, []category.Category{category.Cookie})
	assert.Equal(t, []snapshot.File{{UserDataDirID: "chrome", UserDataDirPath: path, RelativePath: "Default/Network/Cookies"}}, files,
		"a leftover legacy Cookies never wins")
}

func TestPlanExtractions_FlatLayoutProfile(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "History")
	profile := browser.Profile{Name: "Opera Stable"}
	dir := browser.UserDataDir{ID: "opera", Browser: lookup(t, "opera"), Path: path, Profiles: []browser.Profile{profile}}

	extractions, files, _ := planExtractions(dir, []category.Category{category.History})
	assert.Equal(t, []snapshot.File{{UserDataDirID: "opera", UserDataDirPath: path, RelativePath: "History"}}, files)
	assert.Equal(t, []Extraction{{DirID: "opera", Profile: profile, Category: category.History}}, extractions)
}

// TestPlanExtractions_UnsupportedCategory covers Firefox's credit cards, which it keeps nowhere HBD reads.
func TestPlanExtractions_UnsupportedCategory(t *testing.T) {
	profile := browser.Profile{Name: "a.default", RelativePath: "a.default"}
	dir := browser.UserDataDir{ID: "firefox", Browser: lookup(t, "firefox"), Path: t.TempDir(), Profiles: []browser.Profile{profile}}

	extractions, _, skipped := planExtractions(dir, []category.Category{category.CreditCard})
	unsupported := Extraction{DirID: "firefox", Profile: profile, Category: category.CreditCard}
	assert.Equal(t, []Result{{Extraction: unsupported, Outcome: OutcomeUnsupported}}, skipped)
	assert.Empty(t, extractions)
}

// TestPlanExtractions_CategoryWithoutSourcesRuns covers Safari's passwords: read from the keychain, they are never absent.
func TestPlanExtractions_CategoryWithoutSourcesRuns(t *testing.T) {
	profile := browser.Profile{Name: "default", RelativePath: "Safari"}
	dir := browser.UserDataDir{ID: "safari", Browser: lookup(t, "safari"), Path: t.TempDir(), Profiles: []browser.Profile{profile}}

	extractions, files, skipped := planExtractions(dir, []category.Category{category.Password})
	assert.Equal(t, []Extraction{{DirID: "safari", Profile: profile, Category: category.Password}}, extractions)
	assert.Empty(t, files)
	assert.Empty(t, skipped)
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

func TestPlanExtractions_UnreadableFileIsAnError(t *testing.T) {
	dir := chromeDir(t, "Default")
	lock(t, filepath.Join(dir.Path, "Default"))
	extractions, files, skipped := planExtractions(dir, []category.Category{category.History})

	require.Len(t, skipped, 1)
	assert.Equal(t, OutcomeError, skipped[0].Outcome)
	require.ErrorIs(t, skipped[0].Err, fs.ErrPermission)
	assert.Empty(t, extractions)
	assert.Empty(t, files)
}
