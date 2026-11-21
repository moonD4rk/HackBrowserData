package app_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/golden"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/zipfile"
)

// TestRestoreGolden checks that restore writes what dump writes: the extract golden, with each browser's display name.
func TestRestoreGolden(t *testing.T) {
	data := t.TempDir()
	browsertest.WriteChromiumUserDataDirs(t, data)
	keys := chromiumKeyFile(t)
	want := golden.Read(t, "extract-json.golden")

	t.Run("data-dir", func(t *testing.T) {
		out := restoreGolden(t, app.RestoreFlags{KeyFile: keys, DataDir: data, Output: app.OutputFlags{Dir: t.TempDir()}})
		assert.Equal(t, want, golden.ConcatDir(t, out))
	})

	t.Run("zip", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "results")
		restoreGolden(t, app.RestoreFlags{KeyFile: keys, DataDir: data, Output: app.OutputFlags{Dir: out, Zip: true}})

		entries, err := os.ReadDir(out)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "results.zip", entries[0].Name())

		archive := filepath.Join(out, "results.zip")
		golden.Assert(t, "restore-zip.golden", golden.ZipListing(t, archive))
		unzipped := t.TempDir()
		require.NoError(t, zipfile.Extract(archive, unzipped))
		assert.Equal(t, want, golden.ConcatDir(t, unzipped))
	})

	// An archive v1.1.0 wrote is the same <browser-name>/ layout, which the vaults resolve.
	t.Run("data-zip", func(t *testing.T) {
		dataZip := filepath.Join(t.TempDir(), "data.zip")
		browsertest.ZipDir(t, data, dataZip)

		out := restoreGolden(t, app.RestoreFlags{KeyFile: keys, DataZip: dataZip, Output: app.OutputFlags{Dir: t.TempDir()}})
		assert.Equal(t, want, golden.ConcatDir(t, out))
	})
}

// TestArchiveRestoreRoundTrip archives on one host and restores on another, which must write what dump writes.
func TestArchiveRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	for _, fixture := range browsertest.ChromiumUserDataDirs {
		namedBrowser, ok := browser.Lookup(fixture.Name)
		require.True(t, ok, fixture.Name)
		browsertest.WriteChromiumUserDataDir(t, filepath.Join(home, filepath.FromSlash(namedBrowser.Platform().UserDataDirPaths[0])), fixture.Name)
	}
	archive := filepath.Join(t.TempDir(), "data.zip")
	options, err := app.ValidateArchive(app.ArchiveFlags{Browser: "all", Categories: "all", OutputPath: archive})
	require.NoError(t, err)
	options.HomeDir = home
	_, _, err = app.Archive(context.Background(), options)
	require.NoError(t, err)

	out := restoreGolden(t, app.RestoreFlags{KeyFile: chromiumKeyFile(t), DataZip: archive, Output: app.OutputFlags{Dir: t.TempDir()}})
	assert.Equal(t, golden.Read(t, "extract-json.golden"), golden.ConcatDir(t, out))
}

func chromiumKeyFile(t *testing.T) string {
	t.Helper()
	profiles := make([]string, 0, len(browsertest.ChromiumProfiles))
	for _, fixtureProfile := range browsertest.ChromiumProfiles {
		profiles = append(profiles, fixtureProfile.Dir)
	}
	keyFile := masterkey.KeyFile{
		// A literal rather than the version constant: keys files written by today's dumpkeys must keep restoring.
		Version:   "2",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Host:      masterkey.Host{OS: "windows", Arch: "amd64"},
		Vaults:    []masterkey.Vault{},
	}
	for _, fixture := range browsertest.ChromiumUserDataDirs {
		keyFile.Vaults = append(keyFile.Vaults, masterkey.Vault{
			Browser:  fixture.Name,
			Kind:     "chromium",
			Profiles: profiles,
			Keys:     browsertest.ChromiumKeys,
		})
	}

	var buffer bytes.Buffer
	require.NoError(t, keyFile.Write(&buffer))
	path := filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(path, buffer.Bytes(), 0o600))
	return path
}

func restoreGolden(t *testing.T, flags app.RestoreFlags) string {
	t.Helper()
	if flags.Categories == "" {
		flags.Categories = "all"
	}
	if flags.Output.Format == "" {
		flags.Output.Format = "json"
	}
	flags.Output.MaxStorageValue = app.DefaultMaxStorageValue // match the extract goldens, which cap at the default
	options, err := app.ValidateRestore(flags, nil)
	require.NoError(t, err)

	_, summary, err := app.Restore(context.Background(), options)
	require.NoError(t, err)
	require.NoError(t, summary.Err)
	return flags.Output.Dir
}
