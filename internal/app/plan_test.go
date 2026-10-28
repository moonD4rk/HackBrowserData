package app

import (
	"errors"
	"os"
	"path/filepath"
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

func lookup(t *testing.T, id string) browser.Def {
	t.Helper()
	def, ok := browser.Lookup(id)
	require.True(t, ok, id)
	return def
}

func TestNewPlan(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "Local State")
	touch(t, path, "Default", "Login Data")
	touch(t, path, "Default", "Network", "Cookies")
	touch(t, path, "Default", "History")
	touch(t, path, "Profile 1", "History")
	dir := browser.UserDataDir{
		ID: "chrome", Def: lookup(t, "chrome"), Path: path,
		Profiles: []browser.Profile{{Name: "Default", Dir: "Default"}, {Name: "Profile 1", Dir: "Profile 1"}},
	}
	failed := dirFailure("edge", errors.New("permission denied"))
	req := Request{Categories: []category.Category{category.Password, category.Cookie, category.History, category.Download, category.CreditCard}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}, Failed: []Result{failed}}, req)

	file := func(rel string) snapshot.File { return snapshot.File{DirID: "chrome", From: path, Rel: rel} }
	extraction := func(profile string, c category.Category) Extraction {
		return Extraction{DirID: "chrome", Profile: profile, Category: c}
	}
	skipped := func(profile string, c category.Category, o Outcome) Result {
		return Result{Extraction: extraction(profile, c), Outcome: o}
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
	dir := browser.UserDataDir{ID: "chrome", Def: lookup(t, "chrome"), Path: path, Profiles: []browser.Profile{{Name: "Default", Dir: "Default"}}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Request{Categories: []category.Category{category.Cookie}})
	assert.Equal(t, []snapshot.File{{DirID: "chrome", From: path, Rel: "Default/Cookies"}}, got.Files)
}

func TestNewPlan_FlatLayoutProfile(t *testing.T) {
	path := t.TempDir()
	touch(t, path, "History")
	dir := browser.UserDataDir{ID: "opera", Def: lookup(t, "opera"), Path: path, Profiles: []browser.Profile{{Name: "Opera Stable"}}}

	got := NewPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Request{Categories: []category.Category{category.History}})
	assert.Equal(t, []snapshot.File{{DirID: "opera", From: path, Rel: "History"}}, got.Files)
	assert.Equal(t, []Extraction{{DirID: "opera", Profile: "Opera Stable", Category: category.History}}, got.Extractions)
}

func TestNewPlan_UnsupportedCategory(t *testing.T) {
	dir := browser.UserDataDir{ID: "x", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "Default", Dir: "Default"}}}
	none := func(browser.UserDataDir, browser.Profile) browser.Extractors { return browser.Extractors{} }

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Request{Categories: []category.Category{category.CreditCard}}, none)
	assert.Equal(t, []Result{{Extraction: Extraction{DirID: "x", Profile: "Default", Category: category.CreditCard}, Outcome: OutcomeUnsupported}},
		got.Skipped)
	assert.Empty(t, got.Extractions)
}

// Safari's passwords come from the keychain: an extractor that reads no file is never absent.
func TestNewPlan_ExtractorWithoutSourcesRuns(t *testing.T) {
	dir := browser.UserDataDir{ID: "safari", Path: t.TempDir(), Profiles: []browser.Profile{{Name: "default", Dir: "Safari"}}}
	keychainOnly := func(browser.UserDataDir, browser.Profile) browser.Extractors {
		return browser.Extractors{category.Password: {}}
	}

	got := newPlan(Discovery{Dirs: []browser.UserDataDir{dir}}, Request{Categories: []category.Category{category.Password}}, keychainOnly)
	assert.Equal(t, []Extraction{{DirID: "safari", Profile: "default", Category: category.Password}}, got.Extractions)
	assert.Empty(t, got.Files)
	assert.Empty(t, got.Skipped)
}
