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
)

func keysFile(t *testing.T, vaults ...masterkey.Vault) string {
	t.Helper()
	keyFile := masterkey.NewKeyFile()
	keyFile.Vaults = append(keyFile.Vaults, vaults...)
	var buffer bytes.Buffer
	require.NoError(t, keyFile.Write(&buffer))
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
	base := RestoreFlags{KeyFile: keys, DataDir: data, Categories: "all", Format: "json", OutputDir: t.TempDir()}
	for name, edit := range map[string]func(flags *RestoreFlags){
		"no keys":           func(flags *RestoreFlags) { flags.KeyFile = "" },
		"unreadable keys":   func(flags *RestoreFlags) { flags.KeyFile = filepath.Join(data, "missing.json") },
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

	options, keyFile, err := ValidateRestore(RestoreFlags{KeyFile: keys, DataDir: data, Browser: "Chrome", Categories: "all", Format: "json", OutputDir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, "chrome", options.Target.BrowserName)
	assert.Len(t, keyFile.Vaults, 1)
}

func TestDiscoverRestore_SecondUserDataDir(t *testing.T) {
	root := t.TempDir()
	layoutProfile(t, root, "arc")
	layoutProfile(t, root, "arc-2")
	second := testVault("arc")
	second.ID = "arc-2"
	second.Keys = masterkey.Set{masterkey.V10: []byte("arc-2")}
	keyFile := masterkey.KeyFile{Vaults: []masterkey.Vault{testVault("chrome"), testVault("arc"), second}}

	discovery, keys, err := DiscoverRestore(root, keyFile, "arc")
	require.NoError(t, err)
	assert.Equal(t, []browser.UserDataDirID{"arc", "arc-2"}, userDataDirIDs(discovery), "a vault's ID names its directory")
	assert.Equal(t, filepath.Join(root, "arc-2"), discovery.Dirs[1].Path)
	assert.Equal(t, "Arc", discovery.Dirs[1].Browser.DisplayName, "restore sets the display name, as dump does")
	assert.Equal(t, masterkey.Set{masterkey.V10: []byte("arc-2")}, keys["arc-2"])
}

func TestDiscoverRestore_FromLayout(t *testing.T) {
	root := t.TempDir()
	layoutProfile(t, root, "chrome")
	layoutProfile(t, root, "nightly")
	nightly := testVault("Nightly")
	nightly.Kind = "chromium-yandex"
	keyFile := masterkey.KeyFile{Vaults: []masterkey.Vault{testVault("chrome"), nightly, testVault("chrome"), testVault("edge")}}

	discovery, keys, err := DiscoverRestore(root, keyFile, "all")
	require.NoError(t, err)
	assert.Equal(t, []browser.UserDataDirID{"chrome", "nightly"}, userDataDirIDs(discovery), "a repeated vault restores once, one without data not at all")
	assert.Len(t, keys, 2)
	assert.Equal(t, browser.Browser{Name: "yandex", DisplayName: "Nightly", Base: browser.Chromium}, discovery.Dirs[1].Browser,
		"a key this build does not know keeps its kind's format and its own name")
}

func TestDiscoverRestore_OneUserDataNeedsOneVault(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Default", "Preferences")
	touch(t, root, "Default", "History")
	keyFile := masterkey.KeyFile{Vaults: []masterkey.Vault{testVault("chrome"), testVault("edge")}}

	_, _, err := DiscoverRestore(root, keyFile, "all")
	require.Error(t, err)

	discovery, _, err := DiscoverRestore(root, keyFile, "edge")
	require.NoError(t, err)
	require.Equal(t, []browser.UserDataDirID{"edge"}, userDataDirIDs(discovery))
	assert.Equal(t, root, discovery.Dirs[0].Path)
}
