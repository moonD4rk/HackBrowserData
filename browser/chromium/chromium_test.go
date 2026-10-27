package chromium

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
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
		mkFile(fixture.chrome, p, "Preferences")
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
	mkFile(fixture.opera, "Default", "Preferences")
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
	mkFile(fixture.yandex, "Default", "Preferences")
	mkFile(fixture.yandex, "Default", "Ya Passman Data")
	mkFile(fixture.yandex, "Default", "Ya Credit Cards")
	mkFile(fixture.yandex, "Default", "History")
	mkFile(fixture.yandex, "Default", "Network", "Cookies")
	mkFile(fixture.yandex, "Default", "Bookmarks")

	fixture.oldCookies = filepath.Join(fixture.root, "old-cookies")
	mkFile(fixture.oldCookies, "Default", "Preferences")
	mkFile(fixture.oldCookies, "Default", "History")
	mkFile(fixture.oldCookies, "Default", "Cookies")

	fixture.bothCookies = filepath.Join(fixture.root, "both-cookies")
	mkFile(fixture.bothCookies, "Default", "Preferences")
	mkFile(fixture.bothCookies, "Default", "Cookies")
	mkFile(fixture.bothCookies, "Default", "Network", "Cookies")

	fixture.leveldb = filepath.Join(fixture.root, "leveldb")
	mkFile(fixture.leveldb, "Default", "Preferences")
	mkFile(fixture.leveldb, "Default", "History")
	mkDir(fixture.leveldb, "Default", "Local Storage", "leveldb")
	mkFile(fixture.leveldb, "Default", "Local Storage", "leveldb", "000001.ldb")
	mkDir(fixture.leveldb, "Default", "Session Storage")
	mkFile(fixture.leveldb, "Default", "Session Storage", "000001.ldb")

	fixture.leveldbOnly = filepath.Join(fixture.root, "leveldb-only")
	mkFile(fixture.leveldbOnly, "Default", "Preferences")
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
		wantDirs     []category.Category // categories that should be isDir=true
		skipProfiles []string            // should NOT appear
	}{
		{
			name:         "chrome multi-profile",
			dir:          fixture.chrome,
			kind:         types.Chromium,
			wantProfiles: []string{"Default", "Profile 1", "Profile 3"},
			wantCats: map[string][]string{
				"Default": {"Login Data", "Cookies", "History", "Bookmarks", "Web Data", "Secure Preferences", "leveldb", "Session Storage"},
			},
			wantDirs:     []category.Category{category.LocalStorage, category.SessionStorage},
			skipProfiles: []string{"System Profile", "Guest Profile", "Snapshot"},
		},
		{
			name:         "opera with Default",
			dir:          fixture.opera,
			kind:         types.Chromium,
			wantProfiles: []string{"Default"},
			wantCats: map[string][]string{
				"Default": {"Login Data", "History", "Bookmarks", "Cookies"},
			},
		},
		{
			name:         "opera flat layout",
			dir:          fixture.operaFlat,
			kind:         types.Chromium,
			wantProfiles: []string{filepath.Base(fixture.operaFlat)}, // userDataDir itself
			wantCats: map[string][]string{
				filepath.Base(fixture.operaFlat): {"Login Data", "History", "Cookies"},
			},
		},
		{
			name:         "yandex custom files",
			dir:          fixture.yandex,
			kind:         types.ChromiumYandex,
			wantProfiles: []string{"Default"},
			wantCats: map[string][]string{
				"Default": {"Ya Passman Data", "Ya Credit Cards", "History", "Cookies", "Bookmarks"},
			},
		},
		{
			name:         "old cookies fallback",
			dir:          fixture.oldCookies,
			kind:         types.Chromium,
			wantProfiles: []string{"Default"},
		},
		{
			name:         "cookie priority",
			dir:          fixture.bothCookies,
			kind:         types.Chromium,
			wantProfiles: []string{"Default"},
		},
		{
			name:         "leveldb directories",
			dir:          fixture.leveldb,
			kind:         types.Chromium,
			wantProfiles: []string{"Default"},
			wantDirs:     []category.Category{category.LocalStorage, category.SessionStorage},
		},
		{
			name:         "leveldb only",
			dir:          fixture.leveldbOnly,
			kind:         types.Chromium,
			wantProfiles: []string{"Default"},
			wantDirs:     []category.Category{category.LocalStorage, category.SessionStorage},
		},
		{
			name: "empty dir",
			dir:  fixture.empty,
			kind: types.Chromium,
		},
		{
			name: "nonexistent dir",
			dir:  "/nonexistent/path",
			kind: types.Chromium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := types.BrowserConfig{Name: "Test", Kind: tt.kind, UserDataDir: tt.dir}
			b, err := NewBrowser(cfg)
			require.NoError(t, err)

			if len(tt.wantProfiles) == 0 {
				assert.Nil(t, b)
				return
			}
			require.NotNil(t, b)
			require.Len(t, b.profiles, len(tt.wantProfiles))

			nameMap := profilesByName(b)
			assertProfiles(t, nameMap, tt.wantProfiles, tt.skipProfiles)
			assertCategories(t, nameMap, tt.wantCats)
			assertDirCategories(t, b.profiles, tt.wantDirs)
		})
	}
}

// ---------------------------------------------------------------------------
// discoverProfiles: fallback boundaries (marker-less copies, flat-layout precedence)
// ---------------------------------------------------------------------------

func TestDiscoverProfiles(t *testing.T) {
	t.Run("markerless multi-subdir resolves all source-bearing dirs", func(t *testing.T) {
		dir := t.TempDir()
		mkFile(dir, "Default", "History")
		mkFile(dir, "Profile 1", "History")
		assert.Len(t, discoverProfiles(dir, chromiumSources), 2)
	})

	t.Run("marker present keeps fallback dormant", func(t *testing.T) {
		dir := t.TempDir()
		mkFile(dir, "Default", "Preferences")
		mkFile(dir, "Default", "History")
		mkFile(dir, "NotAProfile", "History")
		got := discoverProfiles(dir, chromiumSources)
		require.Len(t, got, 1)
		assert.Equal(t, filepath.Join(dir, "Default"), got[0])
	})

	t.Run("skipped dir ignored by markerless fallback", func(t *testing.T) {
		dir := t.TempDir()
		mkFile(dir, "System Profile", "History")
		assert.Empty(t, discoverProfiles(dir, chromiumSources))
	})

	t.Run("sourceless subdir ignored", func(t *testing.T) {
		dir := t.TempDir()
		mkDir(dir, "Crashpad")
		assert.Empty(t, discoverProfiles(dir, chromiumSources))
	})

	t.Run("flat-layout root wins over source-bearing subdir", func(t *testing.T) {
		dir := t.TempDir()
		mkFile(dir, "History")
		mkFile(dir, "Subthing", "History")
		got := discoverProfiles(dir, chromiumSources)
		require.Len(t, got, 1)
		assert.Equal(t, dir, got[0], "flat-layout root must win; subdir must not hijack discovery")
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func profilesByName(b *Browser) map[string]*profile {
	m := make(map[string]*profile, len(b.profiles))
	for _, p := range b.profiles {
		m[filepath.Base(p.profileDir)] = p
	}
	return m
}

func assertProfiles(t *testing.T, nameMap map[string]*profile, want, skip []string) {
	t.Helper()
	for _, w := range want {
		assert.Contains(t, nameMap, w, "should find profile %s", w)
	}
	for _, s := range skip {
		assert.NotContains(t, nameMap, s, "should skip %s", s)
	}
}

func assertCategories(t *testing.T, nameMap map[string]*profile, wantCats map[string][]string) {
	t.Helper()
	for profileName, wantFiles := range wantCats {
		p, ok := nameMap[profileName]
		if !ok {
			t.Errorf("profile %s not found", profileName)
			continue
		}
		for _, wantFile := range wantFiles {
			found := false
			for _, rp := range p.sourcePaths {
				if filepath.Base(rp.absPath) == wantFile {
					found = true
					break
				}
			}
			assert.True(t, found, "profile %s should have %s", profileName, wantFile)
		}
	}
}

func assertDirCategories(t *testing.T, profiles []*profile, cats []category.Category) {
	t.Helper()
	for _, cat := range cats {
		for _, p := range profiles {
			if rp, ok := p.sourcePaths[cat]; ok {
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
	require.Contains(t, resolved, category.Cookie)
	assert.Contains(t, resolved[category.Cookie].absPath, "Network",
		"Network/Cookies should win over root Cookies")
}

func TestCookieFallback(t *testing.T) {
	resolved := resolveSourcePaths(chromiumSources, filepath.Join(fixture.oldCookies, "Default"))
	require.Contains(t, resolved, category.Cookie)
	assert.NotContains(t, resolved[category.Cookie].absPath, "Network",
		"should fallback to root Cookies when Network/Cookies missing")
}

// ---------------------------------------------------------------------------
// History/Download share the same source file
// ---------------------------------------------------------------------------

func TestSharedSourceFile(t *testing.T) {
	resolved := resolveSourcePaths(chromiumSources, filepath.Join(fixture.chrome, "Default"))
	assert.Equal(t, resolved[category.History].absPath, resolved[category.Download].absPath)
}

// ---------------------------------------------------------------------------
// Source helpers
// ---------------------------------------------------------------------------

func TestSourcesForKind(t *testing.T) {
	chromium := sourcesForKind(types.Chromium)
	yandex := sourcesForKind(types.ChromiumYandex)

	assert.Equal(t, "Login Data", chromium[category.Password][0].rel)
	assert.Equal(t, "Ya Passman Data", yandex[category.Password][0].rel)
	// Yandex inherits non-overridden categories
	assert.Equal(t, chromium[category.History][0].rel, yandex[category.History][0].rel)
}

func TestExtractorsFor(t *testing.T) {
	assert.Equal(t, []string{"Login Data"}, extractorsFor(types.Chromium)[category.Password].Sources[0].Paths)
	assert.Equal(t, []string{"Ya Passman Data"}, extractorsFor(types.ChromiumYandex)[category.Password].Sources[0].Paths)
	assert.Equal(t, []string{"Ya Credit Cards"}, extractorsFor(types.ChromiumYandex)[category.CreditCard].Sources[0].Paths)
}

// The kind still selects a fork's own format: Opera keeps its extensions under extensions.opsettings.
func TestExtract_OperaKindReadsOpsettings(t *testing.T) {
	dir := t.TempDir()
	mkFile(dir, "Default", "Preferences")
	installFile(t, filepath.Join(dir, "Default"), createTestJSON(t, "Secure Preferences", `{
		"extensions": {"opsettings": {"ext1": {"location": 1, "manifest": {"name": "Ext", "version": "1.0"}}}}
	}`), "Secure Preferences")

	b, err := NewBrowser(types.BrowserConfig{Name: "Opera", Kind: types.ChromiumOpera, UserDataDir: dir})
	require.NoError(t, err)
	require.NotNil(t, b)
	b.SetKeys(masterkey.Set{})

	results, err := b.Extract([]category.Category{category.Extension})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Len(t, results[0].Data.Extensions, 1)
	assert.Equal(t, "Ext", results[0].Data.Extensions[0].Name)
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
			b, err := NewBrowser(types.BrowserConfig{Name: "Test", Kind: types.Chromium, UserDataDir: tt.dir})
			require.NoError(t, err)
			require.NotNil(t, b)

			for _, p := range b.profiles {
				localState := filepath.Join(filepath.Dir(p.profileDir), "Local State")
				if tt.want {
					assert.FileExists(t, localState)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Keys
// ---------------------------------------------------------------------------

// Live key retrieval (per-version first-key-wins, ABE dispatch) is tested in the masterkey package;
// here the Browser only has to carry the keys a dump hands it and surface an empty Set when it has none.

func TestMasterKeys_StaticFromDump(t *testing.T) {
	keys := masterkey.Set{masterkey.V10: []byte("k10"), masterkey.V20: []byte("k20")}
	b, err := NewBrowser(types.BrowserConfig{Name: "Test", Kind: types.Chromium, UserDataDir: fixture.chrome})
	require.NoError(t, err)
	require.NotNil(t, b)

	b.SetKeys(keys)
	assert.Equal(t, keys, b.masterKeys())
}

func TestMasterKeys_EmptyDumpYieldsEmptySet(t *testing.T) {
	b, err := NewBrowser(types.BrowserConfig{Name: "Test", Kind: types.Chromium, UserDataDir: fixture.chrome})
	require.NoError(t, err)
	require.NotNil(t, b)

	b.SetKeys(masterkey.Set{})
	assert.Empty(t, b.masterKeys(), "a dump with no keys for this browser decrypts nothing, rather than crashing")
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	mkFile(dir, "Default", "Preferences")
	installFile(t, filepath.Join(dir, "Default"), setupHistoryDB(t), "History")

	tests := []struct {
		name string
		keys masterkey.Set
	}{
		{name: "without keys extracts unencrypted data", keys: masterkey.Set{}},
		{name: "with dump keys", keys: masterkey.Set{masterkey.V10: []byte("test-key-16byte0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewBrowser(types.BrowserConfig{
				Name: "Test", Kind: types.Chromium, UserDataDir: dir, KeychainLabel: "Chrome",
			})
			require.NoError(t, err)
			require.NotNil(t, b)

			// Even an empty Set is needed: without SetKeys, Extract queries the host's real secret stores.
			b.SetKeys(tt.keys)

			results, err := b.Extract([]category.Category{category.History})
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.NotNil(t, results[0].Data)
			require.Len(t, results[0].Data.Histories, 3)
			assert.Equal(t, "GitHub", results[0].Data.Histories[0].Title)
		})
	}
}

// ---------------------------------------------------------------------------
// CountEntries
// ---------------------------------------------------------------------------

func TestCountEntries(t *testing.T) {
	dir := t.TempDir()
	mkFile(dir, "Default", "Preferences")
	installFile(t, filepath.Join(dir, "Default"), setupHistoryDB(t), "History")

	b, err := NewBrowser(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	// No retriever set — CountEntries should still work (no decryption needed).
	results, err := b.CountEntries([]category.Category{category.History, category.Download})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, 3, results[0].Counts[category.History])
	// Download uses a different table in the same file; since we only
	// created the urls table (not downloads), the count query will fail
	// gracefully and return 0.
	assert.Equal(t, 0, results[0].Counts[category.Download])
}

func TestCountEntries_NoRetrieverNeeded(t *testing.T) {
	dir := t.TempDir()
	mkFile(dir, "Default", "Preferences")
	// Login Data normally needs master key to extract, but CountEntries skips decryption.
	installFile(t, filepath.Join(dir, "Default"), setupLoginDB(t), "Login Data")

	b, err := NewBrowser(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	// No retriever set — CountEntries succeeds without master key.
	results, err := b.CountEntries([]category.Category{category.Password})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 2, results[0].Counts[category.Password])
}

// *Browser must satisfy the key-injection shape app uses after construction (live Locator or dump keys).
func TestBrowser_SatisfiesKeyManager(t *testing.T) {
	var _ interface {
		SetLocator(masterkey.Locator)
		SetKeys(masterkey.Set)
		ExportKeys() (masterkey.Set, error)
	} = (*Browser)(nil)
}
