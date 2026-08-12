package chromium

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

func createLoginDBAt(t *testing.T, path string, inserts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(loginsSchema)
	require.NoError(t, err)
	for _, stmt := range inserts {
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}
}

// TestExtractCategory_CustomExtractor verifies that extractCategory dispatches
// through a registered extractor instead of the default switch logic.
func TestExtractCategory_CustomExtractor(t *testing.T) {
	// Create a profile with a custom extractor that records it was called
	called := false
	testExtractor := extensionExtractor{
		fn: func(path string) ([]types.ExtensionEntry, error) {
			called = true
			return []types.ExtensionEntry{{Name: "custom", ID: "test-id"}}, nil
		},
	}

	p := &profile{
		extractors: map[types.Category]categoryExtractor{
			types.Extension: testExtractor,
		},
	}

	data := &types.BrowserData{}
	p.extractCategory(data, types.Extension, masterkey.MasterKeys{}, "unused-path")

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

	p := &profile{
		extractors: nil, // no custom extractors
	}

	data := &types.BrowserData{}
	p.extractCategory(data, types.History, masterkey.MasterKeys{}, path)

	require.Len(t, data.Histories, 1)
	assert.Equal(t, "Example", data.Histories[0].Title)
}

// ---------------------------------------------------------------------------
// acquireFiles
// ---------------------------------------------------------------------------

func TestAcquireFiles(t *testing.T) {
	profileDir := filepath.Join(fixture.chrome, "Default")
	resolved := resolveSourcePaths(chromiumSources, profileDir)

	p := &profile{profileDir: profileDir, sourcePaths: resolved}

	session, err := filemanager.NewSession()
	require.NoError(t, err)
	defer session.Cleanup()

	cats := []types.Category{types.History, types.Cookie, types.Bookmark}
	paths := p.acquireFiles(session, cats)

	assert.Len(t, paths, len(cats))
	for _, path := range paths {
		_, err := os.Stat(path)
		require.NoError(t, err, "acquired file should exist")
	}
}

func TestExtractAndCountPasswords_MergesLocalAndAccountDatabases(t *testing.T) {
	profileDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "Preferences"), []byte(`{}`), 0o600))
	createLoginDBAt(t, filepath.Join(profileDir, "Login Data"),
		insertLogin("https://local.example", "", "local-user", "", 13340000000000000))
	createLoginDBAt(t, filepath.Join(profileDir, accountLoginData),
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))

	p := &profile{
		profileDir:  profileDir,
		browserName: "Chrome",
		kind:        types.Chromium,
		sourcePaths: resolveSourcePaths(chromiumSources, profileDir),
	}
	data := p.extract(masterkey.MasterKeys{}, []types.Category{types.Password})
	require.Len(t, data.Passwords, 2)
	assert.Equal(t, "account-user", data.Passwords[0].Username)
	assert.Equal(t, types.PasswordStoreAccount, data.Passwords[0].Store)
	assert.Equal(t, "local-user", data.Passwords[1].Username)
	assert.Equal(t, types.PasswordStoreLocal, data.Passwords[1].Store)
	assert.Equal(t, 2, p.count([]types.Category{types.Password})[types.Password])
}

// A credential synced to both stores is reported twice; the Store field is what tells them
// apart, so it must survive the merge rather than being deduplicated away.
func TestExtractPasswords_IdenticalCredentialInBothStoresKeepsBoth(t *testing.T) {
	profileDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "Preferences"), []byte(`{}`), 0o600))
	login := insertLogin("https://same.example", "", "same-user", "", 13340000000000000)
	createLoginDBAt(t, filepath.Join(profileDir, "Login Data"), login)
	createLoginDBAt(t, filepath.Join(profileDir, accountLoginData), login)

	p := &profile{
		profileDir:  profileDir,
		browserName: "Chrome",
		kind:        types.Chromium,
		sourcePaths: resolveSourcePaths(chromiumSources, profileDir),
	}
	data := p.extract(masterkey.MasterKeys{}, []types.Category{types.Password})

	require.Len(t, data.Passwords, 2)
	stores := []string{data.Passwords[0].Store, data.Passwords[1].Store}
	assert.ElementsMatch(t, []string{types.PasswordStoreLocal, types.PasswordStoreAccount}, stores)
	assert.Equal(t, 2, p.count([]types.Category{types.Password})[types.Password])
}

// A profile that only ever synced passwords has no local DB, so the account DB wins candidate
// resolution and must not also be picked up as a second source.
func TestExtractAndCountPasswords_AccountDatabaseOnly(t *testing.T) {
	profileDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "Preferences"), []byte(`{}`), 0o600))
	createLoginDBAt(t, filepath.Join(profileDir, accountLoginData),
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))

	p := &profile{
		profileDir:  profileDir,
		browserName: "Chrome",
		kind:        types.Chromium,
		sourcePaths: resolveSourcePaths(chromiumSources, profileDir),
	}
	data := p.extract(masterkey.MasterKeys{}, []types.Category{types.Password})

	require.Len(t, data.Passwords, 1)
	assert.Equal(t, "account-user", data.Passwords[0].Username)
	assert.Equal(t, types.PasswordStoreAccount, data.Passwords[0].Store)
	assert.Equal(t, 1, p.count([]types.Category{types.Password})[types.Password])
}

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := setupHistoryDB(t)
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 3, p.countCategory(types.History, path))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := setupCookieDB(t)
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 2, p.countCategory(types.Cookie, path))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := setupBookmarkJSON(t)
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 3, p.countCategory(types.Bookmark, path))
	})

	t.Run("Extension_Opera", func(t *testing.T) {
		path := createTestJSON(t, "Secure Preferences", `{
			"extensions": {
				"opsettings": {
					"ext1": {"location": 1, "manifest": {"name": "Ext", "version": "1.0"}}
				}
			}
		}`)
		p := &profile{kind: types.ChromiumOpera}
		assert.Equal(t, 1, p.countCategory(types.Extension, path))
	})

	t.Run("FileNotFound", func(t *testing.T) {
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 0, p.countCategory(types.History, "/nonexistent/path"))
	})
}
