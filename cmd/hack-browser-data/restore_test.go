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
)

// restoreFixture zips the Chromium fixture dirs and writes the key file that opens them.
func restoreFixture(t *testing.T) (keys, dataZip string) {
	t.Helper()
	userData := t.TempDir()
	browsertest.WriteChromiumUserDataDirs(t, userData)
	dataZip = filepath.Join(t.TempDir(), "data.zip")
	browsertest.ZipDir(t, userData, dataZip)

	keyFile := masterkey.NewKeyFile()
	for _, fixture := range browsertest.ChromiumUserDataDirs {
		keyFile.Vaults = append(keyFile.Vaults, masterkey.Vault{Browser: fixture.Name, Kind: "chromium", Keys: browsertest.ChromiumKeys})
	}
	var buffer bytes.Buffer
	require.NoError(t, keyFile.Write(&buffer))
	keys = filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(keys, buffer.Bytes(), 0o600))
	return keys, dataZip
}

// TestRunRestore_FromADataZip runs the command from its flags to the file it writes.
func TestRunRestore_FromADataZip(t *testing.T) {
	keys, dataZip := restoreFixture(t)
	out := t.TempDir()
	flags := app.RestoreFlags{KeyFile: keys, DataZip: dataZip, Categories: "cookie", Output: app.OutputFlags{Format: "json", Dir: out}}
	require.NoError(t, runRestore(context.Background(), io.Discard, flags))
	cookies, err := os.ReadFile(filepath.Join(out, "cookie.json"))
	require.NoError(t, err)
	assert.Contains(t, string(cookies), `"value": "theme=dark"`)
}

// TestRunRestore_FailsWhenItsOutputCannotBeWritten: the run's own error is in its Summary, which the command returns.
func TestRunRestore_FailsWhenItsOutputCannotBeWritten(t *testing.T) {
	keys, dataZip := restoreFixture(t)
	out := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(out, "cookie.json"), 0o700))
	flags := app.RestoreFlags{KeyFile: keys, DataZip: dataZip, Categories: "cookie", Output: app.OutputFlags{Format: "json", Dir: out}}
	require.Error(t, runRestore(context.Background(), io.Discard, flags))
}
