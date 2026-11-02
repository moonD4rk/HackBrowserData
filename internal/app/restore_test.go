package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func keysFile(t *testing.T, vaults ...masterkey.Vault) string {
	t.Helper()
	dump := masterkey.NewDump()
	dump.Vaults = append(dump.Vaults, vaults...)
	var buffer bytes.Buffer
	require.NoError(t, dump.WriteJSON(&buffer))
	path := filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(path, buffer.Bytes(), 0o600))
	return path
}

func testVault(browserName string) masterkey.Vault {
	return masterkey.Vault{Browser: browserName, Kind: "chromium", Keys: masterkey.Set{masterkey.V10: []byte(browserName)}}
}

func layoutProfile(t *testing.T, root, relativePath string) {
	t.Helper()
	touch(t, root, filepath.FromSlash(relativePath), "Default", "Preferences")
	touch(t, root, filepath.FromSlash(relativePath), "Default", "History")
}

func TestValidateRestore_RejectsBadInput(t *testing.T) {
	keys := keysFile(t, testVault("chrome"))
	data := t.TempDir()
	base := RestoreFlags{Keys: keys, DataDir: data, Categories: "all", Format: "json", Dir: t.TempDir()}
	for name, edit := range map[string]func(flags *RestoreFlags){
		"no keys":           func(flags *RestoreFlags) { flags.Keys = "" },
		"unreadable keys":   func(flags *RestoreFlags) { flags.Keys = filepath.Join(data, "missing.json") },
		"no data":           func(flags *RestoreFlags) { flags.DataDir = "" },
		"both data inputs":  func(flags *RestoreFlags) { flags.DataZip = filepath.Join(data, "data.zip") },
		"missing data dir":  func(flags *RestoreFlags) { flags.DataDir = filepath.Join(data, "missing") },
		"browser not in it": func(flags *RestoreFlags) { flags.Browser = "edge" },
		"unknown format":    func(flags *RestoreFlags) { flags.Format = "xml" },
	} {
		flags := base
		edit(&flags)
		_, _, err := ValidateRestore(flags)
		require.Error(t, err, name)
	}

	options, dump, err := ValidateRestore(RestoreFlags{Keys: keys, DataDir: data, Browser: "Chrome", Categories: "all", Format: "json", Dir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, "chrome", options.Browser)
	assert.Len(t, dump.Vaults, 1)
}

func TestRestoreDiscovery_FromManifest(t *testing.T) {
	root := t.TempDir()
	data := snapshot.Open(root)
	require.NoError(t, data.WriteManifest(snapshot.Manifest{Version: snapshot.ManifestVersion, Dirs: []snapshot.ManifestDir{
		{ID: "chrome", Browser: "chrome", Base: "chromium", Profiles: []snapshot.ManifestProfile{{Name: "Default", Dir: "Default"}}},
		{ID: "arc-2", Browser: "arc", Base: "chromium", Profiles: []snapshot.ManifestProfile{{Name: "Work", Dir: "Profile 1"}}},
		{ID: "../outside", Browser: "chrome", Base: "chromium"},
		{ID: "edge", Browser: "edge", Base: "chromium", Profiles: []snapshot.ManifestProfile{{Name: "Default", Dir: "../../etc"}}},
	}}))
	arc2 := testVault("arc")
	arc2.ID = "arc-2"
	dump := masterkey.Dump{Vaults: []masterkey.Vault{testVault("chrome"), testVault("arc"), arc2, testVault("edge")}}

	discovery, keys, err := RestoreDiscovery(data, dump, Options{Browser: "all"})
	require.NoError(t, err)
	assert.Equal(t, []browser.UserDataDirID{"chrome", "arc-2"}, userDataDirIDs(discovery), "a manifest path that leaves its directory is refused")
	assert.Equal(t, "Chrome", discovery.Dirs[0].Browser.DisplayName, "restore sets the display name, as dump does")
	assert.Equal(t, filepath.Join(root, "arc-2"), discovery.Dirs[1].Path)
	assert.Equal(t, []browser.Profile{{Name: "Work", Dir: "Profile 1"}}, discovery.Dirs[1].Profiles)
	assert.Equal(t, masterkey.Set{masterkey.V10: []byte("arc")}, keys["arc-2"])

	discovery, _, err = RestoreDiscovery(data, dump, Options{Browser: "arc"})
	require.NoError(t, err)
	assert.Equal(t, []browser.UserDataDirID{"arc-2"}, userDataDirIDs(discovery))
}

func TestRestoreDiscovery_FromLayout(t *testing.T) {
	root := t.TempDir()
	layoutProfile(t, root, "chrome")
	layoutProfile(t, root, "nightly")
	nightly := testVault("Nightly")
	nightly.Kind = "chromium-yandex"
	dump := masterkey.Dump{Vaults: []masterkey.Vault{testVault("chrome"), nightly, testVault("chrome"), testVault("edge")}}

	discovery, keys, err := RestoreDiscovery(snapshot.Open(root), dump, Options{Browser: "all"})
	require.NoError(t, err)
	assert.Equal(t, []browser.UserDataDirID{"chrome", "nightly"}, userDataDirIDs(discovery), "a repeated vault restores once, one without data not at all")
	assert.Len(t, keys, 2)
	assert.Equal(t, browser.Browser{Name: "yandex", DisplayName: "Nightly", Base: browser.Chromium}, discovery.Dirs[1].Browser,
		"a key this build does not know keeps its kind's format and its own name")
}

func TestRestoreDiscovery_OneUserDataNeedsOneVault(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Default", "Preferences")
	touch(t, root, "Default", "History")
	dump := masterkey.Dump{Vaults: []masterkey.Vault{testVault("chrome"), testVault("edge")}}

	_, _, err := RestoreDiscovery(snapshot.Open(root), dump, Options{Browser: "all"})
	require.Error(t, err)

	discovery, _, err := RestoreDiscovery(snapshot.Open(root), dump, Options{Browser: "edge"})
	require.NoError(t, err)
	require.Equal(t, []browser.UserDataDirID{"edge"}, userDataDirIDs(discovery))
	assert.Equal(t, root, discovery.Dirs[0].Path)
}
