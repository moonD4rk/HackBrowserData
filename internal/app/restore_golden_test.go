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
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/golden"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// TestRestoreGolden checks that restore writes what dump writes: the extract golden, with each browser's display name.
func TestRestoreGolden(t *testing.T) {
	data := t.TempDir()
	browsertest.WriteChromiumUserDataDirs(t, data)
	keys := chromiumKeyFile(t)
	want := golden.Read(t, "extract-json.golden")

	t.Run("data-dir", func(t *testing.T) {
		out := restoreGolden(t, app.RestoreFlags{KeyFile: keys, DataDir: data, OutputDir: t.TempDir()})
		assert.Equal(t, want, golden.ConcatDir(t, out))
	})

	t.Run("zip", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "results")
		restoreGolden(t, app.RestoreFlags{KeyFile: keys, DataDir: data, OutputDir: out, Zip: true})

		entries, err := os.ReadDir(out)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "results.zip", entries[0].Name())

		archive := filepath.Join(out, "results.zip")
		golden.Assert(t, "restore-zip.golden", golden.ZipListing(t, archive))
		unzipped, err := snapshot.Unzip(archive)
		require.NoError(t, err)
		defer unzipped.Close()
		assert.Equal(t, want, golden.ConcatDir(t, unzipped.Path()))
	})

	// An archive v1.1.0 wrote is the same <browser-name>/ layout, which the vaults resolve.
	t.Run("data-zip", func(t *testing.T) {
		dataZip := filepath.Join(t.TempDir(), "data.zip")
		require.NoError(t, snapshot.Open(data).Zip(dataZip))

		out := restoreGolden(t, app.RestoreFlags{KeyFile: keys, DataZip: dataZip, OutputDir: t.TempDir()})
		assert.Equal(t, want, golden.ConcatDir(t, out))
	})
}

func chromiumKeyFile(t *testing.T) string {
	t.Helper()
	profiles := make([]string, 0, len(browsertest.ChromiumProfiles))
	for _, fixtureProfile := range browsertest.ChromiumProfiles {
		profiles = append(profiles, fixtureProfile.Dir)
	}
	keyFile := masterkey.KeyFile{
		// A literal rather than KeyFileVersion: keys files written by today's dumpkeys must keep restoring.
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
	if flags.Format == "" {
		flags.Format = "json"
	}
	options, keyFile, err := app.ValidateRestore(flags)
	require.NoError(t, err)

	data := snapshot.Open(flags.DataDir)
	if flags.DataZip != "" {
		data, err = snapshot.Unzip(flags.DataZip)
		require.NoError(t, err)
	}
	defer data.Close()

	discovery, keys, err := app.DiscoverRestore(data.Path(), keyFile, options)
	require.NoError(t, err)
	options.ImportedKeys = keys
	_, err = app.Execute(context.Background(), app.NewPlan(discovery, options), options)
	require.NoError(t, err)
	return flags.OutputDir
}
