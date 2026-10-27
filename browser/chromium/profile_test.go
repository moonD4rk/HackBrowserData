package chromium

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
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

	cats := []category.Category{category.History, category.Cookie, category.Bookmark}
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
		insertLogin("https://local.example", "", "local-user", 13340000000000000))
	createLoginDBAt(t, filepath.Join(profileDir, accountLoginData),
		insertLogin("https://account.example", "", "account-user", 13360000000000000))

	p := &profile{
		profileDir:  profileDir,
		browserName: "Chrome",
		kind:        types.Chromium,
		extractors:  extractorsFor(types.Chromium),
		sourcePaths: resolveSourcePaths(chromiumSources, profileDir),
	}
	data := p.extract(masterkey.Set{}, []category.Category{category.Password})
	require.Len(t, data.Passwords, 2)
	// Account rows are emitted after local ones; the exporter's stable sort relies on that when timestamps tie.
	assert.Equal(t, "local-user", data.Passwords[0].Username)
	assert.Equal(t, record.StoreLocal, data.Passwords[0].Store)
	assert.Equal(t, "account-user", data.Passwords[1].Username)
	assert.Equal(t, record.StoreAccount, data.Passwords[1].Store)
	assert.Equal(t, 2, p.count([]category.Category{category.Password})[category.Password])
}

// A credential synced to both stores is reported twice; the Store field is what tells them
// apart, so it must survive the merge rather than being deduplicated away.
func TestExtractPasswords_IdenticalCredentialInBothStoresKeepsBoth(t *testing.T) {
	profileDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "Preferences"), []byte(`{}`), 0o600))
	login := insertLogin("https://same.example", "", "same-user", 13340000000000000)
	createLoginDBAt(t, filepath.Join(profileDir, "Login Data"), login)
	createLoginDBAt(t, filepath.Join(profileDir, accountLoginData), login)

	p := &profile{
		profileDir:  profileDir,
		browserName: "Chrome",
		kind:        types.Chromium,
		extractors:  extractorsFor(types.Chromium),
		sourcePaths: resolveSourcePaths(chromiumSources, profileDir),
	}
	data := p.extract(masterkey.Set{}, []category.Category{category.Password})

	require.Len(t, data.Passwords, 2)
	stores := []record.Store{data.Passwords[0].Store, data.Passwords[1].Store}
	assert.ElementsMatch(t, []record.Store{record.StoreLocal, record.StoreAccount}, stores)
	assert.Equal(t, 2, p.count([]category.Category{category.Password})[category.Password])
}

// A profile that only ever synced passwords has no local DB, so the account DB wins candidate
// resolution and must not also be picked up as a second source.
func TestExtractAndCountPasswords_AccountDatabaseOnly(t *testing.T) {
	profileDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "Preferences"), []byte(`{}`), 0o600))
	createLoginDBAt(t, filepath.Join(profileDir, accountLoginData),
		insertLogin("https://account.example", "", "account-user", 13360000000000000))

	p := &profile{
		profileDir:  profileDir,
		browserName: "Chrome",
		kind:        types.Chromium,
		extractors:  extractorsFor(types.Chromium),
		sourcePaths: resolveSourcePaths(chromiumSources, profileDir),
	}
	data := p.extract(masterkey.Set{}, []category.Category{category.Password})

	require.Len(t, data.Passwords, 1)
	assert.Equal(t, "account-user", data.Passwords[0].Username)
	assert.Equal(t, record.StoreAccount, data.Passwords[0].Store)
	assert.Equal(t, 1, p.count([]category.Category{category.Password})[category.Password])
}

func TestCountCategory(t *testing.T) {
	t.Run("History", func(t *testing.T) {
		path := setupHistoryDB(t)
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 3, p.countCategory(category.History, path))
	})

	t.Run("Cookie", func(t *testing.T) {
		path := setupCookieDB(t)
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 2, p.countCategory(category.Cookie, path))
	})

	t.Run("Bookmark", func(t *testing.T) {
		path := setupBookmarkJSON(t)
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 3, p.countCategory(category.Bookmark, path))
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
		assert.Equal(t, 1, p.countCategory(category.Extension, path))
	})

	t.Run("FileNotFound", func(t *testing.T) {
		p := &profile{kind: types.Chromium}
		assert.Equal(t, 0, p.countCategory(category.History, "/nonexistent/path"))
	})
}
