package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// TestRunRestore_FromADataZip goes through the command's own unzip, which the app restore goldens do themselves.
func TestRunRestore_FromADataZip(t *testing.T) {
	userData := t.TempDir()
	browsertest.WriteChromiumUserDataDirs(t, userData)
	dataZip := filepath.Join(t.TempDir(), "data.zip")
	require.NoError(t, snapshot.Open(userData).Zip(dataZip))

	keyFile := masterkey.NewKeyFile()
	for _, fixture := range browsertest.ChromiumUserDataDirs {
		keyFile.Vaults = append(keyFile.Vaults, masterkey.Vault{Browser: fixture.Name, Kind: "chromium", Keys: browsertest.ChromiumKeys})
	}
	var buffer bytes.Buffer
	require.NoError(t, keyFile.Write(&buffer))
	keys := filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(keys, buffer.Bytes(), 0o600))

	out := t.TempDir()
	flags := app.RestoreFlags{KeyFile: keys, DataZip: dataZip, Categories: "cookie", Format: "json", OutputDir: out}
	require.NoError(t, runRestore(context.Background(), io.Discard, flags))
	cookies, err := os.ReadFile(filepath.Join(out, "cookie.json"))
	require.NoError(t, err)
	assert.Contains(t, string(cookies), `"value": "theme=dark"`)
}
