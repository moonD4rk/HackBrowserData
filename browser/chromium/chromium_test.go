package chromium

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/masterkey"
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

func assertDirCategories(t *testing.T, profiles []*profile, cats []types.Category) {
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
// getMasterKeys
// ---------------------------------------------------------------------------

// mockRetriever records the arguments passed to RetrieveKey.
type mockRetriever struct {
	hints  masterkey.Hints
	key    []byte
	err    error
	called bool
}

func (m *mockRetriever) RetrieveKey(hints masterkey.Hints) ([]byte, error) {
	m.called = true
	m.hints = hints
	return m.key, m.err
}

func TestGetMasterKeys(t *testing.T) {
	// getMasterKeys routes through masterkey.NewMasterKeys on every platform — the V10 mock
	// wired via SetRetrievers(Retrievers{V10: mock}) is consulted cross-platform.

	// Profile directory without Local State file.
	dirNoLocalState := t.TempDir()
	mkFile(dirNoLocalState, "Default", "Preferences")
	mkFile(dirNoLocalState, "Default", "History")

	tests := []struct {
		name              string
		dir               string
		keychainLabel     string
		retriever         masterkey.Retriever // nil → don't call SetRetrievers
		wantV10           []byte
		wantKeychainLabel string
		wantLocalState    bool // whether localStatePath passed to retriever is non-empty
	}{
		{
			name: "nil retriever yields empty keys",
			dir:  fixture.chrome,
		},
		{
			name:              "with Local State passes path to retriever",
			dir:               fixture.chrome,
			keychainLabel:     "Chrome",
			retriever:         &mockRetriever{key: []byte("fake-master-key")},
			wantV10:           []byte("fake-master-key"),
			wantKeychainLabel: "Chrome",
			wantLocalState:    true,
		},
		{
			name:              "without Local State passes empty path",
			dir:               dirNoLocalState,
			keychainLabel:     "Chromium",
			retriever:         &mockRetriever{key: []byte("derived-key")},
			wantV10:           []byte("derived-key"),
			wantKeychainLabel: "Chromium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewBrowser(types.BrowserConfig{
				Name: "Test", Kind: types.Chromium, UserDataDir: tt.dir, KeychainLabel: tt.keychainLabel,
			})
			require.NoError(t, err)
			require.NotNil(t, b)

			if tt.retriever != nil {
				b.SetRetrievers(masterkey.Retrievers{V10: tt.retriever})
			}

			mk := b.masterKeys()
			assert.Equal(t, tt.wantV10, mk.V10)
			assert.Nil(t, mk.V11, "V11 stays nil when no v11 retriever is wired")
			assert.Nil(t, mk.V20, "V20 stays nil when no v20 retriever is wired")

			if tt.retriever == nil {
				return
			}
			mock, ok := tt.retriever.(*mockRetriever)
			require.True(t, ok)
			assert.True(t, mock.called)
			assert.Equal(t, tt.wantKeychainLabel, mock.hints.KeychainLabel)
			if tt.wantLocalState {
				assert.NotEmpty(t, mock.hints.LocalStatePath)
			} else {
				assert.Empty(t, mock.hints.LocalStatePath)
			}
		})
	}
}

// TestGetMasterKeys_AllTiersInvoked is the mixed-tier regression test at the getMasterKeys layer.
// Before the refactor a Windows-only bypass meant only one tier's retriever was consulted, so a
// profile mixing prefixes silently lost the un-retrieved tier. After the refactor every
// configured tier must be called exactly once and its key must land in the matching MasterKeys
// slot. This catches any future "bypass the masterkey package for a faster path" regression and covers the
// analogous Linux v10/v11 case — no platform silently drops a tier any more.
func TestGetMasterKeys_AllTiersInvoked(t *testing.T) {
	v10mock := &mockRetriever{key: []byte("fake-v10-key")}
	v11mock := &mockRetriever{key: []byte("fake-v11-key")}
	v20mock := &mockRetriever{key: []byte("fake-v20-key")}

	b, err := NewBrowser(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: fixture.chrome, KeychainLabel: "Chrome",
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	b.SetRetrievers(masterkey.Retrievers{V10: v10mock, V11: v11mock, V20: v20mock})

	mk := b.masterKeys()
	assert.Equal(t, []byte("fake-v10-key"), mk.V10, "V10 slot must be populated")
	assert.Equal(t, []byte("fake-v11-key"), mk.V11, "V11 slot must be populated")
	assert.Equal(t, []byte("fake-v20-key"), mk.V20, "V20 slot must be populated")
	assert.True(t, v10mock.called, "V10 retriever must be called — no silent bypass")
	assert.True(t, v11mock.called, "V11 retriever must be called — no silent bypass")
	assert.True(t, v20mock.called, "V20 retriever must be called — no silent bypass")
	for _, m := range []*mockRetriever{v10mock, v11mock, v20mock} {
		assert.Equal(t, "Chrome", m.hints.KeychainLabel)
		assert.NotEmpty(t, m.hints.LocalStatePath, "Local State path must be passed to every retriever")
	}
}

// TestGetMasterKeys_WindowsABEThreading pins cfg.WindowsABE → hints.WindowsABEKey threading. A
// regression here silently disables Windows ABE decryption with no dev-box-test signal — only the
// windows-tunnel sandbox 574-cookie regression would catch it — so it must be pinned at unit level.
func TestGetMasterKeys_WindowsABEThreading(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		windowsABE bool
		wantABEKey string
	}{
		{"WindowsABE=true threads cfg.Key into hints.WindowsABEKey", "chrome", true, "chrome"},
		{"WindowsABE=false leaves hints.WindowsABEKey empty", "opera", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRetriever{key: []byte("k")}
			b, err := NewBrowser(types.BrowserConfig{
				Key: tt.key, Name: "Test", Kind: types.Chromium,
				UserDataDir: fixture.chrome, WindowsABE: tt.windowsABE,
			})
			require.NoError(t, err)
			require.NotNil(t, b)

			b.SetRetrievers(masterkey.Retrievers{V20: mock})

			b.masterKeys()
			assert.Equal(t, tt.wantABEKey, mock.hints.WindowsABEKey)
		})
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
		retriever     masterkey.Retriever // nil → don't call SetRetriever
		wantRetriever bool                // whether retriever should be called
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
			b, err := NewBrowser(types.BrowserConfig{
				Name: "Test", Kind: types.Chromium, UserDataDir: dir, KeychainLabel: "Chrome",
			})
			require.NoError(t, err)
			require.NotNil(t, b)

			if tt.retriever != nil {
				b.SetRetrievers(masterkey.Retrievers{V10: tt.retriever})
			}

			results, err := b.Extract([]types.Category{types.History})
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.NotNil(t, results[0].Data)
			require.Len(t, results[0].Data.Histories, 3)
			// setupHistoryDB: Example(200) > GitHub(100) > Go Dev(50)
			assert.Equal(t, "Example", results[0].Data.Histories[0].Title)

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

	b, err := NewBrowser(types.BrowserConfig{
		Name: "Test", Kind: types.Chromium, UserDataDir: dir,
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	// No retriever set — CountEntries should still work (no decryption needed).
	results, err := b.CountEntries([]types.Category{types.History, types.Download})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, 3, results[0].Counts[types.History])
	// Download uses a different table in the same file; since we only
	// created the urls table (not downloads), the count query will fail
	// gracefully and return 0.
	assert.Equal(t, 0, results[0].Counts[types.Download])
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
	results, err := b.CountEntries([]types.Category{types.Password})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 2, results[0].Counts[types.Password])
}

// ---------------------------------------------------------------------------
// SetRetrievers: verify *Browser satisfies the interface used by
// browser.discoverFromConfigs for post-construction retriever injection.
// ---------------------------------------------------------------------------

func TestSetRetrievers_SatisfiesInterface(t *testing.T) {
	var _ interface {
		SetRetrievers(masterkey.Retrievers)
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
