package firefox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/types"
)

// Shared fixtures built once for all tests.
var fixture struct {
	root       string
	multiProf  string // two Firefox profiles + non-profile dir
	singleProf string // one profile with all data files
	partial    string // profile missing some files
	empty      string
}

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "firefox-test-*")
	if err != nil {
		panic(err)
	}
	fixture.root = root
	buildFixtures()
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

func buildFixtures() {
	allFiles := []string{
		"places.sqlite", "cookies.sqlite", "logins.json",
		"extensions.json", "webappsstore.sqlite", "key4.db",
	}

	// Multi-profile: two valid profiles + one non-profile directory
	fixture.multiProf = filepath.Join(fixture.root, "multi")
	for _, prof := range []string{"abc123.default-release", "xyz789.default"} {
		for _, f := range allFiles {
			mkFile(fixture.multiProf, prof, f)
		}
	}
	mkDir(fixture.multiProf, "Crash Reports")
	mkDir(fixture.multiProf, "Pending Pings")

	// Single profile: one profile with all files
	fixture.singleProf = filepath.Join(fixture.root, "single")
	for _, f := range allFiles {
		mkFile(fixture.singleProf, "m1n2o3.default-release", f)
	}

	// Partial profile: only places.sqlite (no logins, no cookies)
	fixture.partial = filepath.Join(fixture.root, "partial")
	mkFile(fixture.partial, "p4q5r6.default", "places.sqlite")

	// Empty directory
	fixture.empty = filepath.Join(fixture.root, "empty")
	mkDir(fixture.empty)
}

func mkFile(parts ...string) {
	path := filepath.Join(parts...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		panic(err)
	}
}

func mkDir(parts ...string) {
	if err := os.MkdirAll(filepath.Join(parts...), 0o755); err != nil {
		panic(err)
	}
}

// TestNewBrowsers is table-driven, covering all profile discovery scenarios.
func TestNewBrowsers(t *testing.T) {
	tests := []struct {
		name         string
		dir          string
		wantProfiles []string // expected profile base names
		skipDirs     []string // should NOT appear as profiles
	}{
		{
			name:         "multi-profile discovery",
			dir:          fixture.multiProf,
			wantProfiles: []string{"abc123.default-release", "xyz789.default"},
			skipDirs:     []string{"Crash Reports", "Pending Pings"},
		},
		{
			name:         "single profile",
			dir:          fixture.singleProf,
			wantProfiles: []string{"m1n2o3.default-release"},
		},
		{
			name:         "partial profile",
			dir:          fixture.partial,
			wantProfiles: []string{"p4q5r6.default"},
		},
		{
			name: "empty dir",
			dir:  fixture.empty,
		},
		{
			name: "nonexistent dir",
			dir:  "/nonexistent/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := types.BrowserConfig{Name: "Firefox", Kind: types.Firefox, UserDataDir: tt.dir}
			b, err := NewBrowser(cfg)
			require.NoError(t, err)

			if len(tt.wantProfiles) == 0 {
				assert.Nil(t, b)
				return
			}
			require.NotNil(t, b)
			require.Len(t, b.profiles, len(tt.wantProfiles))

			profileNames := make(map[string]bool)
			for _, p := range b.profiles {
				profileNames[filepath.Base(p.profileDir)] = true
			}
			for _, want := range tt.wantProfiles {
				assert.True(t, profileNames[want], "should find profile %s", want)
			}
			for _, skip := range tt.skipDirs {
				assert.False(t, profileNames[skip], "should not find %s", skip)
			}
		})
	}
}

// TestResolveSourcePaths verifies that source resolution correctly maps
// categories to files, including shared files (places.sqlite).
func TestResolveSourcePaths(t *testing.T) {
	profileDir := filepath.Join(fixture.singleProf, "m1n2o3.default-release")
	resolved := resolveSourcePaths(firefoxSources, profileDir)

	// All categories should be resolved
	for _, cat := range []category.Category{
		category.Password, category.Cookie, category.History,
		category.Download, category.Bookmark, category.Extension, category.LocalStorage,
	} {
		assert.Contains(t, resolved, cat, "should resolve %s", cat)
	}

	// History, Download, Bookmark share places.sqlite
	assert.Equal(t, resolved[category.History].absPath, resolved[category.Download].absPath)
	assert.Equal(t, resolved[category.History].absPath, resolved[category.Bookmark].absPath)

	// Password is a different file
	assert.NotEqual(t, resolved[category.Password].absPath, resolved[category.History].absPath)
}

func TestResolveSourcePaths_Partial(t *testing.T) {
	profileDir := filepath.Join(fixture.partial, "p4q5r6.default")
	resolved := resolveSourcePaths(firefoxSources, profileDir)

	// Only places.sqlite exists → History, Download, Bookmark resolved
	assert.Contains(t, resolved, category.History)
	assert.Contains(t, resolved, category.Download)
	assert.Contains(t, resolved, category.Bookmark)

	// No logins.json, cookies.sqlite, etc.
	assert.NotContains(t, resolved, category.Password)
	assert.NotContains(t, resolved, category.Cookie)
	assert.NotContains(t, resolved, category.Extension)
}

// ---------------------------------------------------------------------------
// CountEntries
// ---------------------------------------------------------------------------

func TestCountEntries(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "test-profile")
	mkDir(profileDir)
	installFile(t, profileDir, setupMozHistoryDB(t), "places.sqlite")

	b, err := NewBrowser(types.BrowserConfig{
		Name: "Firefox", Kind: types.Firefox, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	// CountEntries works without master key.
	results, err := b.CountEntries([]category.Category{category.History})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 3, results[0].Counts[category.History])
}
