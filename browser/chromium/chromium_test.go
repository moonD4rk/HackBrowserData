package chromium

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
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
		wantDirs     []types.Category    // categories that should be isDir=true
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
			wantDirs:     []types.Category{types.LocalStorage, types.SessionStorage},
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
			wantDirs:     []types.Category{types.LocalStorage, types.SessionStorage},
		},
		{
			name:         "leveldb only",
			dir:          fixture.leveldbOnly,
			kind:         types.Chromium,
			wantProfiles: []string{"Default"},
			wantDirs:     []types.Category{types.LocalStorage, types.SessionStorage},
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
	chromium := sourcesForKind(types.Chromium)
	yandex := sourcesForKind(types.ChromiumYandex)

	assert.Equal(t, "Login Data", chromium[types.Password][0].rel)
	assert.Equal(t, "Ya Passman Data", yandex[types.Password][0].rel)
	// Yandex inherits non-overridden categories
	assert.Equal(t, chromium[types.History][0].rel, yandex[types.History][0].rel)
}

func TestExtractorsForKind(t *testing.T) {
	assert.Nil(t, extractorsForKind(types.Chromium))

	yandexExt := extractorsForKind(types.ChromiumYandex)
	require.NotNil(t, yandexExt)
	assert.Contains(t, yandexExt, types.Password)
	assert.Contains(t, yandexExt, types.CreditCard)

	operaExt := extractorsForKind(types.ChromiumOpera)
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
	b.extractCategory(data, types.Extension, keyretriever.MasterKeys{}, "unused-path")

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
	b.extractCategory(data, types.History, keyretriever.MasterKeys{}, path)

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
			browsers, err := NewBrowsers(types.BrowserConfig{Name: "Test", Kind: types.Chromium, UserDataDir: tt.dir})
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

// ---------------------------------------------------------------------------
// getMasterKeys
// ---------------------------------------------------------------------------

// mockRetriever records the arguments passed to RetrieveKey.
type mockRetriever struct {
	storage    string
	localState string
	key        []byte
	err        error
	called     bool
}

func (m *mockRetriever) RetrieveKey(storage, localStatePath string) ([]byte, error) {
	m.called = true
	m.storage = storage
	m.localState = localStatePath
	return m.key, m.err
}

func TestGetMasterKeys(t *testing.T) {
	// getMasterKeys routes through keyretriever.NewMasterKeys on every platform — the V10 mock
	// wired via SetKeyRetrievers(Retrievers{V10: mock}) is consulted cross-platform.

	// Profile directory without Local State file.
	dirNoLocalState := t.TempDir()
	mkFile(dirNoLocalState, "Default", "Preferences")
	mkFile(dirNoLocalState, "Default", "History")

	tests := []struct {
		name           string
		dir            string
		storage        string
		retriever      keyretriever.KeyRetriever // nil → don't call SetKeyRetrievers
		wantV10        []byte
		wantStorage    string
		wantLocalState bool // whether localStatePath passed to retriever is non-empty
	}{
		{
			name: "nil retriever yields empty keys",
			dir:  fixture.chrome,
		},
		{
			name:           "with Local State passes path to retriever",
			dir:            fixture.chrome,
			storage:        "Chrome",
			retriever:      &mockRetriever{key: []byte("fake-master-key")},
			wantV10:        []byte("fake-master-key"),
			wantStorage:    "Chrome",
			wantLocalState: true,
		},
		{
			name:        "without Local State passes empty path",
			dir:         dirNoLocalState,
			storage:     "Chromium",
			retriever:   &mockRetriever{key: []byte("derived-key")},
			wantV10:     []byte("derived-key"),
			wantStorage: "Chromium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := NewBrowsers(types.BrowserConfig{
				Name: "Test", Kind: types.Chromium, UserDataDir: tt.dir, Storage: tt.storage,
			})
			require.NoError(t, err)
			require.NotEmpty(t, browsers)

			b := browsers[0]
			if tt.retriever != nil {
				b.SetKeyRetrievers(keyretriever.Retrievers{V10: tt.retriever})
			}

			session, err := filemanager.NewSession()
			require.NoError(t, err)
			defer session.Cleanup()

			keys := b.getMasterKeys(session)
			assert.Equal(t, tt.wantV10, keys.V10)
			assert.Nil(t, keys.V11, "V11 stays nil when no v11 retriever is wired")
			assert.Nil(t, keys.V20, "V20 stays nil when no v20 retriever is wired")

			if tt.retriever == nil {
				return
			}
			mock, ok := tt.retriever.(*mockRetriever)
			require.True(t, ok)
			assert.True(t, mock.called)
			assert.Equal(t, tt.wantStorage, mock.storage)
			if tt.wantLocalState {
				assert.NotEmpty(t, mock.localState)
			} else {
				assert.Empty(t, mock.localState)
			}
		})
	}
}

// TestGetMasterKeys_AllTiersInvoked is the mixed-tier regression test at the getMasterKeys layer.
// Before the refactor a Windows-only bypass meant only one tier's retriever was consulted, so a
// profile mixing prefixes silently lost the un-retrieved tier. After the refactor every
// configured tier must be called exactly once and its key must land in the matching MasterKeys
// slot. This catches any future "bypass keyretriever for a faster path" regression and covers the
// analogous Linux v10/v11 case — no platform silently drops a tier any more.
func TestGetMasterKeys_AllTiersInvoked(t *testing.T) {
	v10mock := &mockRetriever{key: []byte("fake-v10-key")}
	v11mock := &mockRetriever{key: []byte("fake-v11-key")}
	v20mock := &mockRetriever{key: []byte("fake-v20-key")}

	browsers, err := NewBrowsers(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: fixture.chrome, Storage: "Chrome",
	})
	require.NoError(t, err)
	require.NotEmpty(t, browsers)

	b := browsers[0]
	b.SetKeyRetrievers(keyretriever.Retrievers{V10: v10mock, V11: v11mock, V20: v20mock})

	session, err := filemanager.NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	keys := b.getMasterKeys(session)
	assert.Equal(t, []byte("fake-v10-key"), keys.V10, "V10 slot must be populated")
	assert.Equal(t, []byte("fake-v11-key"), keys.V11, "V11 slot must be populated")
	assert.Equal(t, []byte("fake-v20-key"), keys.V20, "V20 slot must be populated")
	assert.True(t, v10mock.called, "V10 retriever must be called — no silent bypass")
	assert.True(t, v11mock.called, "V11 retriever must be called — no silent bypass")
	assert.True(t, v20mock.called, "V20 retriever must be called — no silent bypass")
	for _, m := range []*mockRetriever{v10mock, v11mock, v20mock} {
		assert.Equal(t, "Chrome", m.storage)
		assert.NotEmpty(t, m.localState, "Local State path must be passed to every retriever")
	}
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	mkFile(dir, "Default", "Preferences")
	installFile(t, filepath.Join(dir, "Default"), setupHistoryDB(t), "History")

	tests := []struct {
		name          string
		retriever     keyretriever.KeyRetriever // nil → don't call SetRetriever
		wantRetriever bool                      // whether retriever should be called
	}{
		{
			name: "without retriever extracts unencrypted data",
		},
		{
			name:          "with mock retriever",
			retriever:     &mockRetriever{key: []byte("test-key-16bytes")},
			wantRetriever: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := NewBrowsers(types.BrowserConfig{
				Name: "Test", Kind: types.Chromium, UserDataDir: dir, Storage: "Chrome",
			})
			require.NoError(t, err)
			require.Len(t, browsers, 1)

			if tt.retriever != nil {
				browsers[0].SetKeyRetrievers(keyretriever.Retrievers{V10: tt.retriever})
			}

			result, err := browsers[0].Extract([]types.Category{types.History})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Histories, 3)
			// setupHistoryDB: Example(200) > GitHub(100) > Go Dev(50)
			assert.Equal(t, "Example", result.Histories[0].Title)

			if tt.wantRetriever {
				mock, ok := tt.retriever.(*mockRetriever)
				require.True(t, ok)
				assert.True(t, mock.called)
			}
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

	browsers, err := NewBrowsers(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.Len(t, browsers, 1)

	// No retriever set — CountEntries should still work (no decryption needed).
	counts, err := browsers[0].CountEntries([]types.Category{types.History, types.Download})
	require.NoError(t, err)

	assert.Equal(t, 3, counts[types.History])
	// Download uses a different table in the same file; since we only
	// created the urls table (not downloads), the count query will fail
	// gracefully and return 0.
	assert.Equal(t, 0, counts[types.Download])
}

func TestCountEntries_NoRetrieverNeeded(t *testing.T) {
	dir := t.TempDir()
	mkFile(dir, "Default", "Preferences")
	// Login Data normally needs master key to extract, but CountEntries skips decryption.
	installFile(t, filepath.Join(dir, "Default"), setupLoginDB(t), "Login Data")

	browsers, err := NewBrowsers(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.Len(t, browsers, 1)

	// No retriever set — CountEntries succeeds without master key.
	counts, err := browsers[0].CountEntries([]types.Category{types.Password})
	require.NoError(t, err)
	assert.Equal(t, 2, counts[types.Password])
}

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := setupHistoryDB(t)
		b := &Browser{cfg: types.BrowserConfig{Kind: types.Chromium}}
		assert.Equal(t, 3, b.countCategory(types.History, path))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := setupCookieDB(t)
		b := &Browser{cfg: types.BrowserConfig{Kind: types.Chromium}}
		assert.Equal(t, 2, b.countCategory(types.Cookie, path))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := setupBookmarkJSON(t)
		b := &Browser{cfg: types.BrowserConfig{Kind: types.Chromium}}
		assert.Equal(t, 3, b.countCategory(types.Bookmark, path))
	})

	t.Run("Extension_Opera", func(t *testing.T) {
		path := createTestJSON(t, "Secure Preferences", `{
			"extensions": {
				"opsettings": {
					"ext1": {"location": 1, "manifest": {"name": "Ext", "version": "1.0"}}
				}
			}
		}`)
		b := &Browser{cfg: types.BrowserConfig{Kind: types.ChromiumOpera}}
		assert.Equal(t, 1, b.countCategory(types.Extension, path))
	})

	t.Run("FileNotFound", func(t *testing.T) {
		b := &Browser{cfg: types.BrowserConfig{Kind: types.Chromium}}
		assert.Equal(t, 0, b.countCategory(types.History, "/nonexistent/path"))
	})
}

// ---------------------------------------------------------------------------
// SetKeyRetrievers: verify *Browser satisfies the interface used by
// browser.pickFromConfigs for post-construction retriever injection.
// ---------------------------------------------------------------------------

func TestSetKeyRetrievers_SatisfiesInterface(t *testing.T) {
	var _ interface {
		SetKeyRetrievers(keyretriever.Retrievers)
	} = (*Browser)(nil)
}

// Anchor: 2024-01-15T10:30:00Z as Chromium microseconds since 1601 UTC.
const anchorUnixSeconds = int64(1705314600)

var anchorChromiumMicros = (anchorUnixSeconds + 11644473600) * 1_000_000

func TestTimeEpoch_AnchorDate(t *testing.T) {
	got := timeEpoch(anchorChromiumMicros)
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, want, got)
	assert.Equal(t, anchorUnixSeconds, got.Unix())
}

func TestTimeEpoch_ZeroReturnsZeroTime(t *testing.T) {
	assert.True(t, timeEpoch(0).IsZero())
}

func TestTimeEpoch_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, timeEpoch(-1).IsZero())
}

func TestTimeEpoch_AlwaysUTC(t *testing.T) {
	// assert.Same checks pointer equality: time.UTC and time.Local are
	// distinct *Location globals, so this catches any regression that
	// drops .UTC() even when the runner's TZ happens to be UTC.
	got := timeEpoch(anchorChromiumMicros)
	assert.Same(t, time.UTC, got.Location())
}

func TestTimeEpoch_MicrosecondPrecisionPreserved(t *testing.T) {
	got := timeEpoch(anchorChromiumMicros + 123456)
	assert.Equal(t, 123456*int64(time.Microsecond), int64(got.Nanosecond()))
}

func TestTimeEpoch_UnixEpochBoundary(t *testing.T) {
	got := timeEpoch(chromiumEpochOffsetMicros)
	assert.Equal(t, time.Unix(0, 0).UTC(), got)
}

func TestTimeEpoch_OutOfJSONRangeReturnsZero(t *testing.T) {
	jsonBytes, err := timeEpoch(1 << 62).MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `"0001-01-01T00:00:00Z"`, string(jsonBytes))
}
