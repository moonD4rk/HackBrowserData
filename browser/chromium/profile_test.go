package chromium

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/filemanager"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

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
