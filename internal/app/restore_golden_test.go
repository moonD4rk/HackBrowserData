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
	browsertest.ChromiumInstallations(t, data)
	keys := chromiumFixtureKeysFile(t)
	want := golden.Read(t, "extract-json.golden")

	t.Run("data-dir", func(t *testing.T) {
		out := restoreGolden(t, app.RestoreFlags{Keys: keys, DataDir: data, Dir: t.TempDir()})
		assert.Equal(t, want, golden.Bundle(t, out))
	})

	t.Run("zip", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "results")
		restoreGolden(t, app.RestoreFlags{Keys: keys, DataDir: data, Dir: out, Zip: true})

		entries, err := os.ReadDir(out)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "results.zip", entries[0].Name())

		archive := filepath.Join(out, "results.zip")
		golden.Assert(t, "restore-zip.golden", golden.ZipListing(t, archive))
		unzipped, err := snapshot.Unzip(archive)
		require.NoError(t, err)
		defer unzipped.Close()
		assert.Equal(t, want, golden.Bundle(t, unzipped.Path()))
	})

	// An archive v1.1.0 wrote is the same <browser-name>/ layout, which the vaults resolve.
	t.Run("data-zip", func(t *testing.T) {
		dataZip := filepath.Join(t.TempDir(), "data.zip")
		require.NoError(t, snapshot.Open(data).Zip(dataZip))

		out := restoreGolden(t, app.RestoreFlags{Keys: keys, DataZip: dataZip, Dir: t.TempDir()})
		assert.Equal(t, want, golden.Bundle(t, out))
	})
}

func chromiumFixtureKeysFile(t *testing.T) string {
	t.Helper()
	profiles := make([]string, 0, len(browsertest.ChromiumProfiles))
	for _, fixtureProfile := range browsertest.ChromiumProfiles {
		profiles = append(profiles, fixtureProfile.Dir)
	}
	dump := masterkey.Dump{
		// A literal rather than DumpVersion: keys files written by today's dumpkeys must keep restoring.
		Version:   "2",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Host:      masterkey.Host{OS: "windows", Arch: "amd64"},
		Vaults:    []masterkey.Vault{},
	}
	for _, fixture := range browsertest.ChromiumInstalls {
		dump.Vaults = append(dump.Vaults, masterkey.Vault{
			Browser:  fixture.Name,
			Kind:     "chromium",
			Profiles: profiles,
			Keys:     browsertest.ChromiumKeys,
		})
	}

	var buffer bytes.Buffer
	require.NoError(t, dump.WriteJSON(&buffer))
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
	options, dump, err := app.ValidateRestore(flags)
	require.NoError(t, err)

	data := snapshot.Open(flags.DataDir)
	if flags.DataZip != "" {
		data, err = snapshot.Unzip(flags.DataZip)
		require.NoError(t, err)
	}
	defer data.Close()

	discovery, keys, err := app.RestoreDiscovery(data.Path(), dump, options)
	require.NoError(t, err)
	options.Keys = keys
	_, err = app.Execute(context.Background(), app.NewPlan(discovery, options), options)
	require.NoError(t, err)
	return flags.Dir
}
