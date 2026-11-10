package masterkey_test

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

	"github.com/moond4rk/hackbrowserdata/internal/golden"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

var keysGoldenCreatedAt = time.Date(2026, time.May, 16, 8, 30, 15, 123456789, time.UTC)

// keysFakeKey uses Strict, which rejects non-canonical literals, so keys.golden holds each key as written here.
func keysFakeKey(t *testing.T, encodedKey string) []byte {
	t.Helper()
	key, err := base64.StdEncoding.Strict().DecodeString(encodedKey)
	require.NoError(t, err)
	return key
}

func keysFreeze(t *testing.T, keyFile masterkey.KeyFile, host masterkey.Host) masterkey.KeyFile {
	t.Helper()
	assert.Equal(t, runtime.GOOS, keyFile.Host.OS)
	assert.Equal(t, runtime.GOARCH, keyFile.Host.Arch)
	assert.Equal(t, time.UTC, keyFile.CreatedAt.Location())
	assert.WithinDuration(t, time.Now(), keyFile.CreatedAt, time.Hour)
	keyFile.CreatedAt = keysGoldenCreatedAt
	keyFile.Host = host
	return keyFile
}

// keysVaultsFile has two Chromium installs, the Opera and Yandex forks, and a vault that kept v10 when v20 failed.
func keysVaultsFile(t *testing.T) masterkey.KeyFile {
	t.Helper()
	return masterkey.KeyFile{
		Version:   "2",
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

func keysEmptyFile(t *testing.T) masterkey.KeyFile {
	t.Helper()
	return keysFreeze(t, masterkey.NewKeyFile(), masterkey.Host{OS: "linux", Arch: "arm64"})
}

func TestKeysGolden(t *testing.T) {
	tests := []struct {
		name    string
		golden  string
		keyFile func(t *testing.T) masterkey.KeyFile
	}{
		{name: "vaults", golden: "keys.golden", keyFile: keysVaultsFile},
		{name: "empty", golden: "keys-empty.golden", keyFile: keysEmptyFile},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			var buffer bytes.Buffer
			require.NoError(t, testCase.keyFile(t).Write(&buffer))
			golden.Assert(t, testCase.golden, buffer.String())
		})
	}
}

func TestKeysRead(t *testing.T) {
	tests := []struct {
		name    string
		golden  string
		edit    func(t *testing.T, doc map[string]any)
		want    func(t *testing.T) masterkey.KeyFile
		wantErr string
	}{
		{name: "golden", golden: "keys.golden", want: keysVaultsFile},
		{name: "empty golden", golden: "keys-empty.golden", want: keysEmptyFile},
		{name: "unknown fields", golden: "keys.golden", edit: keysAddUnknownFields, want: keysVaultsFile},
		{name: "older version", golden: "keys.golden", edit: keysSetVersion("1"), wantErr: "unsupported key file version"},
		{name: "newer version", golden: "keys.golden", edit: keysSetVersion("3"), wantErr: "unsupported key file version"},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", testCase.golden))
			require.NoError(t, err)
			if testCase.edit != nil {
				data = keysEditJSON(t, data, testCase.edit)
			}
			got, err := masterkey.ReadKeyFile(bytes.NewReader(data))
			if testCase.wantErr != "" {
				require.ErrorContains(t, err, testCase.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.want(t), got)
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
	for _, value := range vaults {
		vault, ok := value.(map[string]any)
		require.True(t, ok)
		vault["labels"] = []any{"work", "travel"}
	}
}

func keysSetVersion(version string) func(t *testing.T, doc map[string]any) {
	return func(_ *testing.T, doc map[string]any) { doc["version"] = version }
}
