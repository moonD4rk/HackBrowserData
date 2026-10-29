package main

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
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

func archiveLayoutUserData(t *testing.T, root string) {
	t.Helper()
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
	home := t.TempDir()
	def, ok := ibrowser.Lookup(archiveLayoutKey)
	require.True(t, ok)
	archiveLayoutUserData(t, filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0])))

	tests := []struct {
		name       string
		golden     string
		categories string
	}{
		{name: "all", golden: "archive-all.golden", categories: "all"},
		{name: "cookie", golden: "archive-cookie.golden", categories: "cookie"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			req, err := app.ValidateArchive(app.Flags{Browser: archiveLayoutKey, Categories: tt.categories})
			require.NoError(t, err)
			req.Home = home
			out := filepath.Join(t.TempDir(), "browser-data.zip")
			n, err := app.Archive(context.Background(), app.Discover(req), req, out)
			require.NoError(t, err)
			assertGolden(t, tt.golden, fmt.Sprintf("entries=%d\n", n)+zipListing(t, out))

			prefix := archiveLayoutKey + "/"
			names := archiveLayoutEntryNames(t, out)
			assert.Contains(t, names, "manifest.json")
			for _, name := range names {
				if name == "manifest.json" {
					continue
				}
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

// archive on one host and restore on another, both through the CLI, write what dump writes.
func TestArchiveRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	for _, inst := range chromiumFixtureInstalls {
		def, ok := ibrowser.Lookup(inst.key)
		require.True(t, ok, inst.key)
		chromiumFixtureUserData(t, filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0])), inst.key)
	}
	archive := filepath.Join(t.TempDir(), "data.zip")
	runCmd(t, archiveCmd(), "-o", archive)
	require.Contains(t, archiveLayoutEntryNames(t, archive), "manifest.json")

	out := t.TempDir()
	chromiumFixtureRestore(t, "--keys", chromiumFixtureKeysFile(t), "--data-zip", archive, "-d", out)
	assert.Equal(t, readGolden(t, "extract-json.golden"), bundleDir(t, out))
}
