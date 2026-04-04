package chromium

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/types"
)

// ---------------------------------------------------------------------------
// Shared fixture
// ---------------------------------------------------------------------------

var fixture struct {
	root        string
	chrome      string // multi-profile + skipped dirs
	opera       string // has Default/
	operaFlat   string // no Default/, data in root
	yandex      string // Ya Passman Data, Ya Credit Cards
	oldCookies  string // Cookies at root (no Network/)
	bothCookies string // Network/Cookies + Cookies
	leveldb     string // Local Storage/leveldb + Session Storage
	leveldbOnly string // only LevelDB dirs, no files
	empty       string
}

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "chromium-test-*")
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
	fixture.chrome = filepath.Join(fixture.root, "chrome")
	mkFile(fixture.chrome, "Local State")
	for _, p := range []string{"Default", "Profile 1", "Profile 3"} {
		mkFile(fixture.chrome, p, "Login Data")
		mkFile(fixture.chrome, p, "History")
		mkFile(fixture.chrome, p, "Bookmarks")
		mkFile(fixture.chrome, p, "Web Data")
		mkFile(fixture.chrome, p, "Secure Preferences")
		mkFile(fixture.chrome, p, "Network", "Cookies")
		mkDir(fixture.chrome, p, "Local Storage", "leveldb")
		mkDir(fixture.chrome, p, "Session Storage")
	}
	mkFile(fixture.chrome, "System Profile", "History")
	mkFile(fixture.chrome, "Guest Profile", "History")
	mkFile(fixture.chrome, "Snapshot", "Default", "History")

	fixture.opera = filepath.Join(fixture.root, "opera")
	mkFile(fixture.opera, "Local State")
	mkFile(fixture.opera, "Default", "Login Data")
	mkFile(fixture.opera, "Default", "History")
	mkFile(fixture.opera, "Default", "Bookmarks")
	mkFile(fixture.opera, "Default", "Cookies")

	fixture.operaFlat = filepath.Join(fixture.root, "opera-flat")
	mkFile(fixture.operaFlat, "Local State")
	mkFile(fixture.operaFlat, "Login Data")
	mkFile(fixture.operaFlat, "History")
	mkFile(fixture.operaFlat, "Cookies")

	fixture.yandex = filepath.Join(fixture.root, "yandex")
	mkFile(fixture.yandex, "Local State")
	mkFile(fixture.yandex, "Default", "Ya Passman Data")
	mkFile(fixture.yandex, "Default", "Ya Credit Cards")
	mkFile(fixture.yandex, "Default", "History")
	mkFile(fixture.yandex, "Default", "Network", "Cookies")
	mkFile(fixture.yandex, "Default", "Bookmarks")

	fixture.oldCookies = filepath.Join(fixture.root, "old-cookies")
	mkFile(fixture.oldCookies, "Default", "History")
	mkFile(fixture.oldCookies, "Default", "Cookies")

	fixture.bothCookies = filepath.Join(fixture.root, "both-cookies")
	mkFile(fixture.bothCookies, "Default", "Cookies")
	mkFile(fixture.bothCookies, "Default", "Network", "Cookies")

	fixture.leveldb = filepath.Join(fixture.root, "leveldb")
	mkFile(fixture.leveldb, "Default", "History")
	mkDir(fixture.leveldb, "Default", "Local Storage", "leveldb")
	mkFile(fixture.leveldb, "Default", "Local Storage", "leveldb", "000001.ldb")
	mkDir(fixture.leveldb, "Default", "Session Storage")
	mkFile(fixture.leveldb, "Default", "Session Storage", "000001.ldb")

	fixture.leveldbOnly = filepath.Join(fixture.root, "leveldb-only")
	mkDir(fixture.leveldbOnly, "Default", "Local Storage", "leveldb")
	mkDir(fixture.leveldbOnly, "Default", "Session Storage")

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

// ---------------------------------------------------------------------------
// NewBrowsers: table-driven, covers all layouts end-to-end
// ---------------------------------------------------------------------------

func TestNewBrowsers(t *testing.T) {
	tests := []struct {
		name         string
		dir          string
		kind         types.BrowserKind
		wantProfiles []string            // expected profile base names
		wantCats     map[string][]string // profile → expected category base names (spot check)
		wantDirs     []types.Category    // categories that should be isDir=true
		skipProfiles []string            // should NOT appear
	}{
		{
			name:         "chrome multi-profile",
			dir:          fixture.chrome,
			kind:         types.KindChromium,
			wantProfiles: []string{"Default", "Profile 1", "Profile 3"},
			wantCats: map[string][]string{
				"Default": {"Login Data", "Cookies", "History", "Bookmarks", "Web Data", "Secure Preferences", "leveldb", "Session Storage"},
			},
			wantDirs:     []types.Category{types.LocalStorage, types.SessionStorage},
			skipProfiles: []string{"System Profile", "Guest Profile", "Snapshot"},
		},
		{
			name:         "opera with Default",
			dir:          fixture.opera,
			kind:         types.KindChromium,
			wantProfiles: []string{"Default"},
			wantCats: map[string][]string{
				"Default": {"Login Data", "History", "Bookmarks", "Cookies"},
			},
		},
		{
			name:         "opera flat layout",
			dir:          fixture.operaFlat,
			kind:         types.KindChromium,
			wantProfiles: []string{filepath.Base(fixture.operaFlat)}, // userDataDir itself
			wantCats: map[string][]string{
				filepath.Base(fixture.operaFlat): {"Login Data", "History", "Cookies"},
			},
		},
		{
			name:         "yandex custom files",
			dir:          fixture.yandex,
			kind:         types.KindChromiumYandex,
			wantProfiles: []string{"Default"},
			wantCats: map[string][]string{
				"Default": {"Ya Passman Data", "Ya Credit Cards", "History", "Cookies", "Bookmarks"},
			},
		},
		{
			name:         "old cookies fallback",
			dir:          fixture.oldCookies,
			kind:         types.KindChromium,
			wantProfiles: []string{"Default"},
		},
		{
			name:         "cookie priority",
			dir:          fixture.bothCookies,
			kind:         types.KindChromium,
			wantProfiles: []string{"Default"},
		},
		{
			name:         "leveldb directories",
			dir:          fixture.leveldb,
			kind:         types.KindChromium,
			wantProfiles: []string{"Default"},
			wantDirs:     []types.Category{types.LocalStorage, types.SessionStorage},
		},
		{
			name:         "leveldb only",
			dir:          fixture.leveldbOnly,
			kind:         types.KindChromium,
			wantProfiles: []string{"Default"},
			wantDirs:     []types.Category{types.LocalStorage, types.SessionStorage},
		},
		{
			name: "empty dir",
			dir:  fixture.empty,
			kind: types.KindChromium,
		},
		{
			name: "nonexistent dir",
			dir:  "/nonexistent/path",
			kind: types.KindChromium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := types.BrowserConfig{Name: "Test", Kind: tt.kind, UserDataDir: tt.dir}
			browsers, err := NewBrowsers(cfg)
			require.NoError(t, err)

			if len(tt.wantProfiles) == 0 {
				assert.Empty(t, browsers)
				return
			}
			require.Len(t, browsers, len(tt.wantProfiles))

			nameMap := browsersByProfile(browsers)
			assertProfiles(t, nameMap, tt.wantProfiles, tt.skipProfiles)
			assertCategories(t, nameMap, tt.wantCats)
			assertDirCategories(t, browsers, tt.wantDirs)
		})
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func browsersByProfile(browsers []*Browser) map[string]*Browser {
	m := make(map[string]*Browser, len(browsers))
	for _, b := range browsers {
		m[filepath.Base(b.profileDir)] = b
	}
	return m
}

func assertProfiles(t *testing.T, nameMap map[string]*Browser, want, skip []string) {
	t.Helper()
	for _, w := range want {
		assert.Contains(t, nameMap, w, "should find profile %s", w)
	}
	for _, s := range skip {
		assert.NotContains(t, nameMap, s, "should skip %s", s)
	}
}

func assertCategories(t *testing.T, nameMap map[string]*Browser, wantCats map[string][]string) {
	t.Helper()
	for profileName, wantFiles := range wantCats {
		b, ok := nameMap[profileName]
		if !ok {
			t.Errorf("profile %s not found", profileName)
			continue
		}
		for _, wantFile := range wantFiles {
			found := false
			for _, rp := range b.sourcePaths {
				if filepath.Base(rp.absPath) == wantFile {
					found = true
					break
				}
			}
			assert.True(t, found, "profile %s should have %s", profileName, wantFile)
		}
	}
}

func assertDirCategories(t *testing.T, browsers []*Browser, cats []types.Category) {
	t.Helper()
	for _, cat := range cats {
		for _, b := range browsers {
			if rp, ok := b.sourcePaths[cat]; ok {
				assert.True(t, rp.isDir, "%s should be isDir=true", cat)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Cookie priority: Network/Cookies wins over root Cookies
// ---------------------------------------------------------------------------

func TestCookiePriority(t *testing.T) {
	resolved := resolveSourcePaths(chromiumSources, filepath.Join(fixture.bothCookies, "Default"))
	require.Contains(t, resolved, types.Cookie)
	assert.Contains(t, resolved[types.Cookie].absPath, "Network",
		"Network/Cookies should win over root Cookies")
}

func TestCookieFallback(t *testing.T) {
	resolved := resolveSourcePaths(chromiumSources, filepath.Join(fixture.oldCookies, "Default"))
	require.Contains(t, resolved, types.Cookie)
	assert.NotContains(t, resolved[types.Cookie].absPath, "Network",
		"should fallback to root Cookies when Network/Cookies missing")
}

// ---------------------------------------------------------------------------
// History/Download share the same source file
// ---------------------------------------------------------------------------

func TestSharedSourceFile(t *testing.T) {
	resolved := resolveSourcePaths(chromiumSources, filepath.Join(fixture.chrome, "Default"))
	assert.Equal(t, resolved[types.History].absPath, resolved[types.Download].absPath)
}

// ---------------------------------------------------------------------------
// Source helpers
// ---------------------------------------------------------------------------

func TestSourcesForKind(t *testing.T) {
	chromium := sourcesForKind(types.KindChromium)
	yandex := sourcesForKind(types.KindChromiumYandex)

	assert.Equal(t, "Login Data", chromium[types.Password][0].rel)
	assert.Equal(t, "Ya Passman Data", yandex[types.Password][0].rel)
	// Yandex inherits non-overridden categories
	assert.Equal(t, chromium[types.History][0].rel, yandex[types.History][0].rel)
}

func TestExtractorsForKind(t *testing.T) {
	assert.Nil(t, extractorsForKind(types.KindChromium))

	yandexExt := extractorsForKind(types.KindChromiumYandex)
	require.NotNil(t, yandexExt)
	assert.Contains(t, yandexExt, types.Password)

	operaExt := extractorsForKind(types.KindChromiumOpera)
	require.NotNil(t, operaExt)
	assert.Contains(t, operaExt, types.Extension)
}

// TestExtractCategory_CustomExtractor verifies that extractCategory dispatches
// through a registered extractor instead of the default switch logic.
func TestExtractCategory_CustomExtractor(t *testing.T) {
	// Create a Browser with a custom extractor that records it was called
	called := false
	testExtractor := extensionExtractor{
		fn: func(path string) ([]types.ExtensionEntry, error) {
			called = true
			return []types.ExtensionEntry{{Name: "custom", ID: "test-id"}}, nil
		},
	}

	b := &Browser{
		extractors: map[types.Category]categoryExtractor{
			types.Extension: testExtractor,
		},
	}

	data := &types.BrowserData{}
	b.extractCategory(data, types.Extension, nil, "unused-path")

	assert.True(t, called, "custom extractor should be called")
	require.Len(t, data.Extensions, 1)
	assert.Equal(t, "custom", data.Extensions[0].Name)
}

// TestExtractCategory_DefaultFallback verifies that extractCategory uses
// the default switch when no extractor is registered.
func TestExtractCategory_DefaultFallback(t *testing.T) {
	path := createTestDB(t, "History", urlsSchema,
		insertURL("https://example.com", "Example", 3, 13350000000000000),
	)

	b := &Browser{
		extractors: nil, // no custom extractors
	}

	data := &types.BrowserData{}
	b.extractCategory(data, types.History, nil, path)

	require.Len(t, data.Histories, 1)
	assert.Equal(t, "Example", data.Histories[0].Title)
}

// ---------------------------------------------------------------------------
// acquireFiles
// ---------------------------------------------------------------------------

func TestAcquireFiles(t *testing.T) {
	profileDir := filepath.Join(fixture.chrome, "Default")
	resolved := resolveSourcePaths(chromiumSources, profileDir)

	b := &Browser{profileDir: profileDir, sources: chromiumSources, sourcePaths: resolved}

	session, err := filemanager.NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	cats := []types.Category{types.History, types.Cookie, types.Bookmark}
	paths := b.acquireFiles(session, cats)

	assert.Len(t, paths, len(cats))
	for _, p := range paths {
		_, err := os.Stat(p)
		require.NoError(t, err, "acquired file should exist")
	}
}

// ---------------------------------------------------------------------------
// Local State path validation
// ---------------------------------------------------------------------------

func TestLocalStatePath(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want bool // Local State should be at Dir(profileDir)/Local State
	}{
		{"chrome", fixture.chrome, true},
		{"opera", fixture.opera, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := NewBrowsers(types.BrowserConfig{Name: "Test", Kind: types.KindChromium, UserDataDir: tt.dir})
			require.NoError(t, err)
			require.NotEmpty(t, browsers)

			for _, b := range browsers {
				localState := filepath.Join(filepath.Dir(b.profileDir), "Local State")
				if tt.want {
					assert.FileExists(t, localState)
				}
			}
		})
	}
}
