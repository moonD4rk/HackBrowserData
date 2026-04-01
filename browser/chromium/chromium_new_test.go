package chromium

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/types"
)

// createTestDBAt creates a test SQLite database at the given absolute path.
func createTestDBAt(t *testing.T, path, schema string, inserts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(schema)
	require.NoError(t, err)
	for _, stmt := range inserts {
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}
}

func TestDiscoverProfiles(t *testing.T) {
	userDataDir := t.TempDir()
	fileNames := map[string]bool{"History": true, "Cookies": true}

	// Create Default profile
	defaultDir := filepath.Join(userDataDir, "Default")
	require.NoError(t, os.MkdirAll(defaultDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(defaultDir, "History"), []byte("h"), 0o644))

	// Create Profile 1
	profile1Dir := filepath.Join(userDataDir, "Profile 1")
	require.NoError(t, os.MkdirAll(profile1Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(profile1Dir, "History"), []byte("h"), 0o644))

	// System Profile should be skipped
	sysDir := filepath.Join(userDataDir, "System Profile")
	require.NoError(t, os.MkdirAll(sysDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sysDir, "History"), []byte("h"), 0o644))

	profiles := discoverProfiles(userDataDir, fileNames)
	assert.Len(t, profiles, 2)
	assert.Contains(t, profiles, defaultDir)
	assert.Contains(t, profiles, profile1Dir)
	assert.NotContains(t, profiles, sysDir)
}

func TestDiscoverProfiles_NetworkCookies(t *testing.T) {
	userDataDir := t.TempDir()
	fileNames := map[string]bool{"Cookies": true}

	// Chrome 130+: Network/Cookies
	defaultDir := filepath.Join(userDataDir, "Default")
	networkDir := filepath.Join(defaultDir, "Network")
	require.NoError(t, os.MkdirAll(networkDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(networkDir, "Cookies"), []byte("c"), 0o644))

	profiles := discoverProfiles(userDataDir, fileNames)
	require.Len(t, profiles, 1)
	// Profile dir should be Default, not Default/Network
	assert.Contains(t, profiles, defaultDir)
	assert.Equal(t, filepath.Join(networkDir, "Cookies"), profiles[defaultDir]["Cookies"])
}

func TestDiscoverProfiles_Empty(t *testing.T) {
	userDataDir := t.TempDir()
	profiles := discoverProfiles(userDataDir, map[string]bool{"History": true})
	assert.Empty(t, profiles)
}

func TestNewBrowsers(t *testing.T) {
	userDataDir := t.TempDir()

	// Create two profiles with History files
	for _, name := range []string{"Default", "Profile 1"} {
		dir := filepath.Join(userDataDir, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		createTestDBAt(t, filepath.Join(dir, "History"), urlsSchema,
			insertURL("https://example.com", "Example", 1, 13340000000000000))
	}

	cfg := types.BrowserConfig{
		Name: "Chrome",
		Kind: types.KindChromium,
	}
	browsers, err := NewBrowsers(cfg, userDataDir)
	require.NoError(t, err)
	require.Len(t, browsers, 2)

	names := map[string]bool{}
	for _, b := range browsers {
		names[b.Name()] = true
	}
	assert.True(t, names["Chrome-Default"])
	assert.True(t, names["Chrome-Profile 1"])
}

func TestNewBrowsers_NoProfiles(t *testing.T) {
	browsers, err := NewBrowsers(types.BrowserConfig{Kind: types.KindChromium}, t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, browsers)
}

func TestAcquireFiles(t *testing.T) {
	profileDir := t.TempDir()

	// Create source files matching chromiumSources paths
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "History"), []byte("h"), 0o644))
	networkDir := filepath.Join(profileDir, "Network")
	require.NoError(t, os.MkdirAll(networkDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(networkDir, "Cookies"), []byte("c"), 0o644))

	b := &Browser{
		profileDir: profileDir,
		sources:    chromiumSources,
		sourcePaths: map[types.Category]string{
			types.History: filepath.Join(profileDir, "History"),
			types.Cookie:  filepath.Join(networkDir, "Cookies"),
		},
	}

	session, err := filemanager.NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	paths := b.acquireFiles(session, []types.Category{types.History, types.Cookie})
	assert.Contains(t, paths, types.History)
	assert.Contains(t, paths, types.Cookie)

	// Verify temp files exist
	for _, p := range paths {
		_, err := os.Stat(p)
		assert.NoError(t, err)
	}
}

func TestAcquireFiles_CookieFallback(t *testing.T) {
	profileDir := t.TempDir()

	// Old-style Cookies (no Network/ subdirectory)
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "Cookies"), []byte("c"), 0o644))

	b := &Browser{
		profileDir: profileDir,
		sources:    chromiumSources,
		sourcePaths: map[types.Category]string{
			types.Cookie: filepath.Join(profileDir, "Cookies"),
		},
	}

	session, err := filemanager.NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	paths := b.acquireFiles(session, []types.Category{types.Cookie})
	assert.Contains(t, paths, types.Cookie)
}

func TestExtract_NonEncryptedCategories(t *testing.T) {
	profileDir := t.TempDir()

	// Create History DB with test data
	createTestDBAt(t, filepath.Join(profileDir, "History"), urlsSchema,
		insertURL("https://go.dev", "Go", 10, 13340000000000000),
		insertURL("https://github.com", "GitHub", 20, 13350000000000000),
	)

	b := &Browser{
		cfg:        types.BrowserConfig{Name: "Test"},
		name:       "Test-Default",
		profileDir: profileDir,
		sources:    chromiumSources,
		sourcePaths: map[types.Category]string{
			types.History: filepath.Join(profileDir, "History"),
		},
	}

	data, err := b.Extract([]types.Category{types.History})
	require.NoError(t, err)
	require.Len(t, data.Histories, 2)

	// Verify data is sorted (descending by visit count)
	assert.Equal(t, 20, data.Histories[0].VisitCount)
	assert.Equal(t, 10, data.Histories[1].VisitCount)
}

func TestExtract_MissingCategory(t *testing.T) {
	b := &Browser{
		cfg:         types.BrowserConfig{Name: "Test"},
		name:        "Test-Default",
		profileDir:  t.TempDir(),
		sources:     chromiumSources,
		sourcePaths: map[types.Category]string{}, // no files
	}

	data, err := b.Extract([]types.Category{types.History, types.Bookmark})
	require.NoError(t, err)
	assert.Empty(t, data.Histories)
	assert.Empty(t, data.Bookmarks)
}

func TestResolveSourcePaths(t *testing.T) {
	filePaths := map[string]string{
		"History":   "/tmp/Default/History",
		"Cookies":   "/tmp/Default/Network/Cookies",
		"Bookmarks": "/tmp/Default/Bookmarks",
	}

	resolved := resolveSourcePaths(chromiumSources, filePaths)

	assert.Equal(t, "/tmp/Default/History", resolved[types.History])
	assert.Equal(t, "/tmp/Default/History", resolved[types.Download]) // same file
	assert.Equal(t, "/tmp/Default/Network/Cookies", resolved[types.Cookie])
	assert.Equal(t, "/tmp/Default/Bookmarks", resolved[types.Bookmark])
}

func TestResolveSourcePaths_CookieFallback(t *testing.T) {
	// Old Chrome: Cookies at profile root, not in Network/
	filePaths := map[string]string{
		"Cookies": "/tmp/Default/Cookies",
	}

	resolved := resolveSourcePaths(chromiumSources, filePaths)
	// Cookie source paths: ["Network/Cookies", "Cookies"]
	// Base("Network/Cookies") = "Cookies", matches
	assert.Equal(t, "/tmp/Default/Cookies", resolved[types.Cookie])
}

func TestSourcesForKind(t *testing.T) {
	chromium := sourcesForKind(types.KindChromium)
	yandex := sourcesForKind(types.KindChromiumYandex)

	// Yandex overrides Password source
	assert.Equal(t, []string{"Login Data"}, chromium[types.Password].paths)
	assert.Equal(t, []string{"Ya Passman Data"}, yandex[types.Password].paths)
}

func TestQueriesForKind(t *testing.T) {
	assert.Nil(t, queriesForKind(types.KindChromium))

	yandexQ := queriesForKind(types.KindChromiumYandex)
	assert.Contains(t, yandexQ[types.Password], "action_url")
}
