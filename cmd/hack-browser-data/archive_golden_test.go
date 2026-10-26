package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/types"
)

const archiveLayoutKey = "chrome"

var archiveLayoutProfileFiles = []string{
	"Preferences",
	"Login Data",
	"Network/Cookies",
	"History",
	"Bookmarks",
	"Web Data",
	"Secure Preferences",
	"Local Storage/leveldb/000003.log",
	"Local Storage/leveldb/CURRENT",
	"Local Storage/leveldb/LOCK",
	"Local Storage/leveldb/MANIFEST-000001",
	"Session Storage/000004.log",
	"Session Storage/CURRENT",
	"Session Storage/LOCK",
	"Session Storage/MANIFEST-000001",
}

var archiveLayoutExtraFiles = []string{
	"Local State",
	"Default/Login Data For Account",
	"Default/Network/Cookies-wal",
	"Default/Network/Cookies-shm",
	"Default/Favicons",
	"System Profile/Preferences",
	"System Profile/History",
}

func archiveLayoutUserData(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := append([]string{}, archiveLayoutExtraFiles...)
	for _, profile := range []string{"Default", "Profile 1"} {
		for _, rel := range archiveLayoutProfileFiles {
			files = append(files, profile+"/"+rel)
		}
	}
	for _, rel := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(rel), 0o600))
	}
	return root
}

func archiveLayoutEntryNames(t *testing.T, zipPath string) []string {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer r.Close()
	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func TestArchiveGolden(t *testing.T) {
	b, err := chromium.NewBrowser(types.BrowserConfig{
		Key:         archiveLayoutKey,
		Name:        "Chrome",
		Kind:        types.Chromium,
		UserDataDir: archiveLayoutUserData(t),
	})
	require.NoError(t, err)
	require.NotNil(t, b)

	tests := []struct {
		name       string
		golden     string
		categories []types.Category
	}{
		{name: "all", golden: "archive-all.golden", categories: types.AllCategories},
		{name: "cookie", golden: "archive-cookie.golden", categories: []types.Category{types.Cookie}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "browser-data.zip")
			n, err := browser.WriteArchive([]browser.Browser{b}, tt.categories, out)
			require.NoError(t, err)
			assertGolden(t, tt.golden, fmt.Sprintf("entries=%d\n", n)+zipListing(t, out))

			prefix := archiveLayoutKey + "/"
			names := archiveLayoutEntryNames(t, out)
			for _, name := range names {
				assert.Truef(t, strings.HasPrefix(name, prefix), "entry %q lacks the %q prefix", name, prefix)
				assert.NotContains(t, name, `\`)
				assert.NotEqual(t, "LOCK", path.Base(name))
				assert.Falsef(t, strings.HasPrefix(name, prefix+"System Profile/"), "entry %q comes from a skipped profile", name)
			}
			assert.Contains(t, names, prefix+"Default/Network/Cookies")
			assert.Contains(t, names, prefix+"Default/Network/Cookies-wal")
			assert.NotContains(t, names, prefix+"Default/Favicons")
		})
	}
}
