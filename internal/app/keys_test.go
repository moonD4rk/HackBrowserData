package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func dirOf(t *testing.T, id browser.DirID, defID string) browser.UserDataDir {
	t.Helper()
	dir := browser.UserDataDir{ID: id, Def: lookup(t, defID), Path: t.TempDir()}
	dir.Profiles = []browser.Profile{{Name: "Default", Dir: "Default"}}
	return dir
}

func TestDumpKeys(t *testing.T) {
	chrome := chromeDir(t, "chrome", "Default", "Profile 1")
	touch(t, chrome.Path, "Local State")
	arc2, opera, vivaldi, brave := dirOf(t, "arc-2", "arc"), dirOf(t, "opera", "opera"), dirOf(t, "vivaldi", "vivaldi"), dirOf(t, "brave", "brave")
	firefox := dirOf(t, "firefox", "firefox")

	keys := func(_ context.Context, dir browser.UserDataDir, snap snapshot.Snapshot) (masterkey.Set, error) {
		switch dir.ID {
		case "chrome":
			_, err := snap.Path("Local State")
			require.NoError(t, err, "key retrieval reads the captured Local State")
		case "vivaldi":
			return nil, errors.New("v10 failed")
		case "brave":
			return masterkey.Set{masterkey.V10: []byte("brave")}, errors.New("v20 failed")
		case "firefox":
			t.Error("Firefox has no keys.json vault")
		}
		return masterkey.Set{masterkey.V10: []byte(dir.ID)}, nil
	}
	d := Discovery{Dirs: []browser.UserDataDir{chrome, firefox, arc2, opera, vivaldi, brave}}
	dump := dumpKeysWith(context.Background(), d, keys)

	assert.Equal(t, []masterkey.Vault{
		{
			Browser: "chrome", Kind: "chromium", UserDataDir: chrome.Path, Profiles: []string{"Default", "Profile 1"},
			Keys: masterkey.Set{masterkey.V10: []byte("chrome")},
		},
		{
			Browser: "arc", ID: "arc-2", Kind: "chromium", UserDataDir: arc2.Path, Profiles: []string{"Default"},
			Keys: masterkey.Set{masterkey.V10: []byte("arc-2")},
		},
		{
			Browser: "opera", Kind: "chromium-opera", UserDataDir: opera.Path, Profiles: []string{"Default"},
			Keys: masterkey.Set{masterkey.V10: []byte("opera")},
		},
		{
			Browser: "brave", Kind: "chromium", UserDataDir: brave.Path, Profiles: []string{"Default"},
			Keys: masterkey.Set{masterkey.V10: []byte("brave")},
		},
	}, dump.Vaults, "a failed dir is left out, a partly failed one keeps what it has")
}
