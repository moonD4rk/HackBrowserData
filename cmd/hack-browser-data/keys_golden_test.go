package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

var keysGoldenCreatedAt = time.Date(2026, time.May, 16, 8, 30, 15, 123456789, time.UTC)

// Strict rejects non-canonical literals, so every key reads in keys.golden exactly as written here.
func keysFakeKey(t *testing.T, b64 string) []byte {
	t.Helper()
	key, err := base64.StdEncoding.Strict().DecodeString(b64)
	require.NoError(t, err)
	return key
}

func keysFreeze(t *testing.T, dump masterkey.Dump, host masterkey.Host) masterkey.Dump {
	t.Helper()
	assert.Equal(t, runtime.GOOS, dump.Host.OS)
	assert.Equal(t, runtime.GOARCH, dump.Host.Arch)
	assert.Equal(t, time.UTC, dump.CreatedAt.Location())
	assert.WithinDuration(t, time.Now(), dump.CreatedAt, time.Hour)
	dump.CreatedAt = keysGoldenCreatedAt
	dump.Host = host
	return dump
}

// keysVaultsDump is what dumpkeys wrote for the golden's machine: two Chromium installs, the Opera and Yandex forks, and
// a vault whose v20 key failed, which keeps its v10 key; a Firefox install and a vault with no key are not in it.
func keysVaultsDump(t *testing.T) masterkey.Dump {
	t.Helper()
	return masterkey.Dump{
		Version:   masterkey.DumpVersion,
		CreatedAt: keysGoldenCreatedAt,
		Host:      masterkey.Host{OS: "windows", Arch: "amd64", Hostname: "GOLDEN-PC", User: `GOLDEN-PC\Jürgen`},
		Vaults: []masterkey.Vault{
			{
				Browser: "chrome", Kind: "chromium", UserDataDir: `C:\Users\Jürgen\AppData\Local\Google\Chrome\User Data`,
				Profiles: []string{"Default", "Profile 1"},
				Keys: masterkey.Set{
					masterkey.V10: keysFakeKey(t, "chrome+v10/os+crypt+key/AAAAAAAAAAAAAAAAAAA="),
					masterkey.V20: keysFakeKey(t, "chrome+v20/app+bound+key/AAAAAAAAAAAAAAAAAA="),
				},
			},
			{
				Browser: "edge", Kind: "chromium", UserDataDir: `C:\Users\Jürgen\AppData\Local\Microsoft\Edge\User Data`,
				Profiles: []string{"Default"},
				Keys: masterkey.Set{
					masterkey.V10: keysFakeKey(t, "edge+v10/os+crypt+key/AAAAAAAAAAAAAAAAAAAAA="),
					masterkey.V20: keysFakeKey(t, "edge+v20/app+bound+key/AAAAAAAAAAAAAAAAAAAA="),
				},
			},
			{
				Browser: "opera", Kind: "chromium-opera", UserDataDir: "/home/r&d/.config/opera",
				Profiles: []string{"Default"},
				Keys: masterkey.Set{
					masterkey.V10: keysFakeKey(t, "opera+v10/peanuts+AAAA=="),
					masterkey.V11: keysFakeKey(t, "opera+v11/keyring+AAAA=="),
				},
			},
			{
				Browser: "yandex", Kind: "chromium-yandex", UserDataDir: "/Users/r&d/Library/Application Support/Yandex/YandexBrowser",
				Profiles: []string{"Default"},
				Keys:     masterkey.Set{masterkey.V10: keysFakeKey(t, "yandex+v10/keychain+AA==")},
			},
			{
				Browser: "brave", Kind: "chromium", UserDataDir: `C:\Users\Jürgen\AppData\Local\BraveSoftware\Brave-Browser\User Data`,
				Profiles: []string{"Default"},
				Keys:     masterkey.Set{masterkey.V10: keysFakeKey(t, "brave+v10/os+crypt+key/AAAAAAAAAAAAAAAAAAAA=")},
			},
		},
	}
}

func keysEmptyDump(t *testing.T) masterkey.Dump {
	t.Helper()
	return keysFreeze(t, masterkey.NewDump(), masterkey.Host{OS: "linux", Arch: "arm64"})
}

func TestKeysGolden(t *testing.T) {
	tests := []struct {
		name   string
		golden string
		dump   func(t *testing.T) masterkey.Dump
	}{
		{name: "vaults", golden: "keys.golden", dump: keysVaultsDump},
		{name: "empty", golden: "keys-empty.golden", dump: keysEmptyDump},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, tt.dump(t).WriteJSON(&buf))
			assertGolden(t, tt.golden, buf.String())
		})
	}
}

func TestKeysRead(t *testing.T) {
	tests := []struct {
		name    string
		golden  string
		edit    func(t *testing.T, doc map[string]any)
		want    func(t *testing.T) masterkey.Dump
		wantErr string
	}{
		{name: "golden", golden: "keys.golden", want: keysVaultsDump},
		{name: "empty golden", golden: "keys-empty.golden", want: keysEmptyDump},
		{name: "unknown fields", golden: "keys.golden", edit: keysAddUnknownFields, want: keysVaultsDump},
		{name: "older version", golden: "keys.golden", edit: keysSetVersion("1"), wantErr: "unsupported dump version"},
		{name: "newer version", golden: "keys.golden", edit: keysSetVersion("3"), wantErr: "unsupported dump version"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(goldenDir, tt.golden))
			require.NoError(t, err)
			if tt.edit != nil {
				data = keysEditJSON(t, data, tt.edit)
			}
			got, err := masterkey.ReadJSON(bytes.NewReader(data))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want(t), got)
		})
	}
}

func keysEditJSON(t *testing.T, data []byte, edit func(t *testing.T, doc map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	edit(t, doc)
	edited, err := json.Marshal(doc)
	require.NoError(t, err)
	return edited
}

func keysAddUnknownFields(t *testing.T, doc map[string]any) {
	doc["generator"] = map[string]any{"name": "hack-browser-data", "version": "v9.9.9"}
	vaults, ok := doc["vaults"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, vaults)
	for _, v := range vaults {
		vault, ok := v.(map[string]any)
		require.True(t, ok)
		vault["labels"] = []any{"work", "travel"}
	}
}

func keysSetVersion(version string) func(t *testing.T, doc map[string]any) {
	return func(_ *testing.T, doc map[string]any) { doc["version"] = version }
}
