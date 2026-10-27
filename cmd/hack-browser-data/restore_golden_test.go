package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

func TestRestoreGolden(t *testing.T) {
	data := t.TempDir()
	chromiumFixtureInstallations(t, data)
	keys := chromiumFixtureKeysFile(t)
	want := restoreWantFiles(t, data)

	t.Run("data-dir", func(t *testing.T) {
		out := t.TempDir()
		chromiumFixtureRestore(t, "--keys", keys, "--data-dir", data, "-d", out)
		assert.Equal(t, want, readDirFiles(t, out))
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
		unzipped := t.TempDir()
		require.NoError(t, fileutil.Unzip(archive, unzipped))
		assert.Equal(t, want, readDirFiles(t, unzipped))
	})

	t.Run("data-zip", func(t *testing.T) {
		dataZip := filepath.Join(t.TempDir(), "data.zip")
		_, err := browser.WriteArchive(chromiumFixtureBrowsers(t, data), category.All(), dataZip)
		require.NoError(t, err)

		out := t.TempDir()
		chromiumFixtureRestore(t, "--keys", keys, "--data-zip", dataZip, "-d", out)
		assert.Equal(t, want, readDirFiles(t, out))
	})
}

// Restore has no golden of its own: its output is extract's json output, pinned by extract-json.golden, restamped.
func restoreWantFiles(t *testing.T, data string) map[string]string {
	t.Helper()
	// Known difference: restore stamps each row with the vault key, extract with the display name.
	stamps := make([]string, 0, 2*len(chromiumFixtureInstalls))
	for _, inst := range chromiumFixtureInstalls {
		stamps = append(stamps, fmt.Sprintf(`"browser": %q`, inst.name), fmt.Sprintf(`"browser": %q`, inst.key))
	}
	toVaultKeys := strings.NewReplacer(stamps...)

	files := readDirFiles(t, extractGoldenRun(t, data, "json"))
	for name, content := range files {
		restamped := toVaultKeys.Replace(content)
		require.NotEqualf(t, content, restamped, "%s carries no display-name stamp to restamp", name)
		files[name] = restamped
	}
	return files
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
	cmd := restoreCmd()
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
}
