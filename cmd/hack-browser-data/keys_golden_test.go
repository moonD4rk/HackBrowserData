package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

var keysGoldenCreatedAt = time.Date(2026, time.May, 16, 8, 30, 15, 123456789, time.UTC)

var (
	_ browser.Browser    = (*keysFakeBrowser)(nil)
	_ browser.KeyManager = (*keysFakeChromium)(nil)
)

type keysFakeBrowser struct {
	name, userDataDir string
	profiles          []string
}

func (b *keysFakeBrowser) BrowserName() string { return b.name }
func (b *keysFakeBrowser) UserDataDir() string { return b.userDataDir }

func (b *keysFakeBrowser) Profiles() []types.Profile {
	out := make([]types.Profile, 0, len(b.profiles))
	for _, name := range b.profiles {
		out = append(out, types.Profile{Name: name, Dir: filepath.Join(b.userDataDir, name)})
	}
	return out
}

func (b *keysFakeBrowser) Extract(_ []category.Category) ([]types.ExtractResult, error) {
	return nil, nil
}

func (b *keysFakeBrowser) CountEntries(_ []category.Category) ([]types.CountResult, error) {
	return nil, nil
}

type keysFakeChromium struct {
	keysFakeBrowser
	key       string
	kind      types.BrowserKind
	keys      masterkey.MasterKeys
	exportErr error
}

func (c *keysFakeChromium) SetRetrievers(_ masterkey.Retrievers) {}

func (c *keysFakeChromium) ExportKeys() (masterkey.MasterKeys, error) { return c.keys, c.exportErr }

func (c *keysFakeChromium) BrowserKey() string { return c.key }

func (c *keysFakeChromium) Kind() types.BrowserKind { return c.kind }

// Strict rejects non-canonical literals, so every key reads in keys.golden exactly as written here.
func keysFakeKey(t *testing.T, b64 string) []byte {
	t.Helper()
	key, err := base64.StdEncoding.Strict().DecodeString(b64)
	require.NoError(t, err)
	return key
}

func keysFakeBrowsers(t *testing.T) []browser.Browser {
	t.Helper()
	return []browser.Browser{
		&keysFakeChromium{
			keysFakeBrowser: keysFakeBrowser{
				name:        "Chrome",
				userDataDir: `C:\Users\Jürgen\AppData\Local\Google\Chrome\User Data`,
				profiles:    []string{"Default", "Profile 1"},
			},
			key:  "chrome",
			kind: types.Chromium,
			keys: masterkey.MasterKeys{
				V10: keysFakeKey(t, "chrome+v10/os+crypt+key/AAAAAAAAAAAAAAAAAAA="),
				V20: keysFakeKey(t, "chrome+v20/app+bound+key/AAAAAAAAAAAAAAAAAA="),
			},
		},
		&keysFakeBrowser{
			name:        "Firefox",
			userDataDir: "/home/r&d/.mozilla/firefox",
			profiles:    []string{"x7k2p9qd.default-release"},
		},
		&keysFakeChromium{
			keysFakeBrowser: keysFakeBrowser{
				name:        "Microsoft Edge",
				userDataDir: `C:\Users\Jürgen\AppData\Local\Microsoft\Edge\User Data`,
				profiles:    []string{"Default"},
			},
			key:  "edge",
			kind: types.Chromium,
			keys: masterkey.MasterKeys{
				V10: keysFakeKey(t, "edge+v10/os+crypt+key/AAAAAAAAAAAAAAAAAAAAA="),
				V20: keysFakeKey(t, "edge+v20/app+bound+key/AAAAAAAAAAAAAAAAAAAA="),
			},
		},
		&keysFakeChromium{
			keysFakeBrowser: keysFakeBrowser{
				name:        "Opera",
				userDataDir: "/home/r&d/.config/opera",
				profiles:    []string{"Default"},
			},
			key:  "opera",
			kind: types.ChromiumOpera,
			keys: masterkey.MasterKeys{
				V10: keysFakeKey(t, "opera+v10/peanuts+AAAA=="),
				V11: keysFakeKey(t, "opera+v11/keyring+AAAA=="),
			},
		},
		&keysFakeChromium{
			keysFakeBrowser: keysFakeBrowser{
				name:        "Vivaldi",
				userDataDir: `C:\Users\Jürgen\AppData\Local\Vivaldi\User Data`,
				profiles:    []string{"Default"},
			},
			key:       "vivaldi",
			kind:      types.Chromium,
			exportErr: errors.New("v10: DPAPI decrypt failed"),
		},
		&keysFakeChromium{
			keysFakeBrowser: keysFakeBrowser{
				name:        "Yandex",
				userDataDir: "/Users/r&d/Library/Application Support/Yandex/YandexBrowser",
				profiles:    []string{"Default"},
			},
			key:  "yandex",
			kind: types.ChromiumYandex,
			keys: masterkey.MasterKeys{V10: keysFakeKey(t, "yandex+v10/keychain+AA==")},
		},
		&keysFakeChromium{
			keysFakeBrowser: keysFakeBrowser{
				name:        "Brave",
				userDataDir: `C:\Users\Jürgen\AppData\Local\BraveSoftware\Brave-Browser\User Data`,
				profiles:    []string{"Default"},
			},
			key:       "brave",
			kind:      types.Chromium,
			keys:      masterkey.MasterKeys{V10: keysFakeKey(t, "brave+v10/os+crypt+key/AAAAAAAAAAAAAAAAAAAA=")},
			exportErr: errors.New("v20: elevation service unavailable"),
		},
	}
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

func keysVaultsDump(t *testing.T) masterkey.Dump {
	t.Helper()
	host := masterkey.Host{OS: "windows", Arch: "amd64", Hostname: "GOLDEN-PC", User: `GOLDEN-PC\Jürgen`}
	return keysFreeze(t, browser.BuildDump(keysFakeBrowsers(t)), host)
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
