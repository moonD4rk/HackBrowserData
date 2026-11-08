package app_test

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
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/golden"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
)

const archiveLayoutBrowser = "chrome"

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
		for _, relativePath := range archiveLayoutProfileFiles {
			files = append(files, profile+"/"+relativePath)
		}
	}
	for _, relativePath := range files {
		absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
		require.NoError(t, os.WriteFile(absolutePath, []byte(relativePath), 0o600))
	}
}

func archiveLayoutEntryNames(t *testing.T, zipPath string) []string {
	t.Helper()
	zipReader, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer zipReader.Close()
	names := make([]string, 0, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		names = append(names, zipFile.Name)
	}
	return names
}

func TestArchiveGolden(t *testing.T) {
	home := t.TempDir()
	namedBrowser, ok := browser.Lookup(archiveLayoutBrowser)
	require.True(t, ok)
	archiveLayoutUserData(t, filepath.Join(home, filepath.FromSlash(namedBrowser.Platform().UserDataDirs[0])))

	tests := []struct {
		name       string
		golden     string
		categories string
	}{
		{name: "all", golden: "archive-all.golden", categories: "all"},
		{name: "cookie", golden: "archive-cookie.golden", categories: "cookie"},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			options, err := app.ValidateArchive(app.ArchiveFlags{Browser: archiveLayoutBrowser, Categories: testCase.categories})
			require.NoError(t, err)
			options.Target.HomeDir = home
			out := filepath.Join(t.TempDir(), "browser-data.zip")
			fileCount, err := app.Archive(context.Background(), app.Discover(options.Target), options.Categories, out)
			require.NoError(t, err)
			golden.Assert(t, testCase.golden, fmt.Sprintf("entries=%d\n", fileCount)+golden.ZipListing(t, out))

			prefix := archiveLayoutBrowser + "/"
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

// TestArchiveRestoreRoundTrip archives on one host and restores on another, which must write what dump writes.
func TestArchiveRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	for _, fixture := range browsertest.ChromiumUserDataDirs {
		namedBrowser, ok := browser.Lookup(fixture.Name)
		require.True(t, ok, fixture.Name)
		browsertest.WriteChromiumUserDataDir(t, filepath.Join(home, filepath.FromSlash(namedBrowser.Platform().UserDataDirs[0])), fixture.Name)
	}
	options, err := app.ValidateArchive(app.ArchiveFlags{Browser: "all", Categories: "all"})
	require.NoError(t, err)
	options.Target.HomeDir = home
	archive := filepath.Join(t.TempDir(), "data.zip")
	_, err = app.Archive(context.Background(), app.Discover(options.Target), options.Categories, archive)
	require.NoError(t, err)

	out := restoreGolden(t, app.RestoreFlags{KeyFile: chromiumKeyFile(t), DataZip: archive, OutputDir: t.TempDir()})
	assert.Equal(t, golden.Read(t, "extract-json.golden"), golden.ConcatDir(t, out))
}
