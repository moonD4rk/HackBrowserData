package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Restore writes what dump writes: the extract golden, stamped with each browser's display name.
func TestRestoreGolden(t *testing.T) {
	data := t.TempDir()
	chromiumFixtureInstallations(t, data)
	keys := chromiumFixtureKeysFile(t)
	want := readGolden(t, "extract-json.golden")

	t.Run("data-dir", func(t *testing.T) {
		out := t.TempDir()
		chromiumFixtureRestore(t, "--keys", keys, "--data-dir", data, "-d", out)
		assert.Equal(t, want, bundleDir(t, out))
	})

	t.Run("zip", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "results")
		chromiumFixtureRestore(t, "--keys", keys, "--data-dir", data, "-d", out, "--zip")

		entries, err := os.ReadDir(out)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "results.zip", entries[0].Name())

		archive := filepath.Join(out, "results.zip")
		assertGolden(t, "restore-zip.golden", zipListing(t, archive))
		unzipped, err := snapshot.Unzip(archive)
		require.NoError(t, err)
		defer unzipped.Close()
		assert.Equal(t, want, bundleDir(t, unzipped.Path()))
	})

	// An archive written before the manifest is the bare <browser-key>/ layout, which the vaults resolve.
	t.Run("data-zip without a manifest", func(t *testing.T) {
		dataZip := filepath.Join(t.TempDir(), "data.zip")
		require.NoError(t, snapshot.Open(data).Zip(dataZip))

		out := t.TempDir()
		chromiumFixtureRestore(t, "--keys", keys, "--data-zip", dataZip, "-d", out)
		assert.Equal(t, want, bundleDir(t, out))
	})
}

func chromiumFixtureKeysFile(t *testing.T) string {
	t.Helper()
	profiles := make([]string, 0, len(chromiumFixtureProfiles))
	for _, p := range chromiumFixtureProfiles {
		profiles = append(profiles, p.dir)
	}
	dump := masterkey.Dump{
		// A literal rather than DumpVersion: keys files written by today's dumpkeys must keep restoring.
		Version:   "2",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Host:      masterkey.Host{OS: "windows", Arch: "amd64"},
		Vaults:    []masterkey.Vault{},
	}
	for _, inst := range chromiumFixtureInstalls {
		dump.Vaults = append(dump.Vaults, masterkey.Vault{
			Browser:  inst.key,
			Kind:     "chromium",
			Profiles: profiles,
			Keys:     chromiumFixtureKeys,
		})
	}

	var buf bytes.Buffer
	require.NoError(t, dump.WriteJSON(&buf))
	path := filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func chromiumFixtureRestore(t *testing.T, args ...string) {
	t.Helper()
	runCmd(t, restoreCmd(), args...)
}
