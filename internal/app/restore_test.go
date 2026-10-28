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
	var buf bytes.Buffer
	require.NoError(t, dump.WriteJSON(&buf))
	path := filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func vault(browserKey string) masterkey.Vault {
	return masterkey.Vault{Browser: browserKey, Kind: "chromium", Keys: masterkey.Set{masterkey.V10: []byte(browserKey)}}
}

// layoutProfile lays out one restorable profile under root/<rel>.
func layoutProfile(t *testing.T, root, rel string) {
	t.Helper()
	touch(t, root, filepath.FromSlash(rel), "Default", "Preferences")
	touch(t, root, filepath.FromSlash(rel), "Default", "History")
}

func TestValidateRestore_RejectsBadInput(t *testing.T) {
	keys := keysFile(t, vault("chrome"))
	data := t.TempDir()
	base := RestoreFlags{Keys: keys, DataDir: data, Categories: "all", Format: "json", Dir: t.TempDir()}
	for name, edit := range map[string]func(f *RestoreFlags){
		"no keys":           func(f *RestoreFlags) { f.Keys = "" },
		"unreadable keys":   func(f *RestoreFlags) { f.Keys = filepath.Join(data, "missing.json") },
		"no data":           func(f *RestoreFlags) { f.DataDir = "" },
		"both data inputs":  func(f *RestoreFlags) { f.DataZip = filepath.Join(data, "data.zip") },
		"missing data dir":  func(f *RestoreFlags) { f.DataDir = filepath.Join(data, "missing") },
		"browser not in it": func(f *RestoreFlags) { f.Browser = "edge" },
		"unknown format":    func(f *RestoreFlags) { f.Format = "xml" },
	} {
		f := base
		edit(&f)
		_, _, err := ValidateRestore(f)
		require.Error(t, err, name)
	}

	req, dump, err := ValidateRestore(RestoreFlags{Keys: keys, DataDir: data, Browser: "Chrome", Categories: "all", Format: "json", Dir: t.TempDir()})
	require.NoError(t, err)
	assert.Equal(t, "chrome", req.Browser)
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
	arc2 := vault("arc")
	arc2.ID = "arc-2"
	dump := masterkey.Dump{Vaults: []masterkey.Vault{vault("chrome"), vault("arc"), arc2, vault("edge")}}

	d, keys, err := RestoreDiscovery(data, dump, Request{Browser: "all"})
	require.NoError(t, err)
	assert.Equal(t, []browser.DirID{"chrome", "arc-2"}, dirIDs(d), "a manifest path that leaves its directory is refused")
	assert.Equal(t, "Chrome", d.Dirs[0].Def.Name, "restore stamps the display name, as dump does")
	assert.Equal(t, filepath.Join(root, "arc-2"), d.Dirs[1].Path)
	assert.Equal(t, []browser.Profile{{Name: "Work", Dir: "Profile 1"}}, d.Dirs[1].Profiles)
	assert.Equal(t, masterkey.Set{masterkey.V10: []byte("arc")}, keys["arc-2"])

	d, _, err = RestoreDiscovery(data, dump, Request{Browser: "arc"})
	require.NoError(t, err)
	assert.Equal(t, []browser.DirID{"arc-2"}, dirIDs(d))
}

func TestRestoreDiscovery_FromLayout(t *testing.T) {
	root := t.TempDir()
	layoutProfile(t, root, "chrome")
	layoutProfile(t, root, "nightly")
	nightly := vault("Nightly")
	nightly.Kind = "chromium-yandex"
	dump := masterkey.Dump{Vaults: []masterkey.Vault{vault("chrome"), nightly, vault("chrome"), vault("edge")}}

	d, keys, err := RestoreDiscovery(snapshot.Open(root), dump, Request{Browser: "all"})
	require.NoError(t, err)
	assert.Equal(t, []browser.DirID{"chrome", "nightly"}, dirIDs(d), "a repeated vault restores once, one without data not at all")
	assert.Len(t, keys, 2)
	assert.Equal(t, browser.Def{ID: "yandex", Name: "Nightly", Base: browser.Chromium}, d.Dirs[1].Def,
		"a key this build does not know keeps its kind's format and its own name")
}

func TestRestoreDiscovery_OneUserDataNeedsOneVault(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Default", "Preferences")
	touch(t, root, "Default", "History")
	dump := masterkey.Dump{Vaults: []masterkey.Vault{vault("chrome"), vault("edge")}}

	_, _, err := RestoreDiscovery(snapshot.Open(root), dump, Request{Browser: "all"})
	require.Error(t, err)

	d, _, err := RestoreDiscovery(snapshot.Open(root), dump, Request{Browser: "edge"})
	require.NoError(t, err)
	require.Equal(t, []browser.DirID{"edge"}, dirIDs(d))
	assert.Equal(t, root, d.Dirs[0].Path)
}
