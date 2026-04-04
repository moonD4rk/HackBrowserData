package browser

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

func mkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o644))
}

func TestListBrowsers(t *testing.T) {
	list := ListBrowsers()
	assert.NotEmpty(t, list)
	assert.True(t, sort.StringsAreSorted(list))
}

func TestPickFromConfigs_NameFilter(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "Default", "Login Data")
	mkFile(t, dir, "Default", "History")

	configs := []types.BrowserConfig{
		{Key: "chrome", Name: "Chrome", Kind: types.KindChromium, UserDataDir: dir},
		{Key: "edge", Name: "Edge", Kind: types.KindChromium, UserDataDir: dir},
	}

	tests := []struct {
		name         string
		pickName     string
		wantNames    []string
		wantProfiles []string
	}{
		{
			name:         "exact match",
			pickName:     "chrome",
			wantNames:    []string{"Chrome"},
			wantProfiles: []string{"Default"},
		},
		{
			name:         "case insensitive",
			pickName:     "Chrome",
			wantNames:    []string{"Chrome"},
			wantProfiles: []string{"Default"},
		},
		{
			name:         "all returns both",
			pickName:     "all",
			wantNames:    []string{"Chrome", "Edge"},
			wantProfiles: []string{"Default", "Default"},
		},
		{
			name:     "unknown returns empty",
			pickName: "safari",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := pickFromConfigs(configs, tt.pickName, "")
			require.NoError(t, err)
			assertBrowsers(t, browsers, tt.wantNames, tt.wantProfiles)
		})
	}
}

func TestPickFromConfigs_BrowserKind(t *testing.T) {
	chromeDir := t.TempDir()
	mkFile(t, chromeDir, "Default", "Login Data")
	mkFile(t, chromeDir, "Default", "History")
	mkFile(t, chromeDir, "Profile 1", "Login Data")
	mkFile(t, chromeDir, "Profile 1", "History")

	firefoxDir := t.TempDir()
	mkFile(t, firefoxDir, "abc123.default-release", "logins.json")
	mkFile(t, firefoxDir, "abc123.default-release", "places.sqlite")

	yandexDir := t.TempDir()
	mkFile(t, yandexDir, "Default", "Ya Passman Data")
	mkFile(t, yandexDir, "Default", "History")

	tests := []struct {
		name         string
		configs      []types.BrowserConfig
		wantNames    []string
		wantProfiles []string
	}{
		{
			name: "chromium multi-profile",
			configs: []types.BrowserConfig{
				{Key: "chrome", Name: "Chrome", Kind: types.KindChromium, UserDataDir: chromeDir},
			},
			wantNames:    []string{"Chrome", "Chrome"},
			wantProfiles: []string{"Default", "Profile 1"},
		},
		{
			name: "firefox random dir",
			configs: []types.BrowserConfig{
				{Key: "firefox", Name: "Firefox", Kind: types.KindFirefox, UserDataDir: firefoxDir},
			},
			wantNames:    []string{"Firefox"},
			wantProfiles: []string{"abc123.default-release"},
		},
		{
			name: "yandex variant",
			configs: []types.BrowserConfig{
				{Key: "yandex", Name: "Yandex", Kind: types.KindChromiumYandex, UserDataDir: yandexDir},
			},
			wantNames:    []string{"Yandex"},
			wantProfiles: []string{"Default"},
		},
		{
			name: "nonexistent dir",
			configs: []types.BrowserConfig{
				{Key: "chrome", Name: "Chrome", Kind: types.KindChromium, UserDataDir: "/nonexistent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := pickFromConfigs(tt.configs, "all", "")
			require.NoError(t, err)
			assertBrowsers(t, browsers, tt.wantNames, tt.wantProfiles)
		})
	}
}

func TestPickFromConfigs_ProfilePath(t *testing.T) {
	chromeDir := t.TempDir()
	mkFile(t, chromeDir, "Default", "Login Data")
	mkFile(t, chromeDir, "Default", "History")
	mkFile(t, chromeDir, "Profile 1", "Login Data")
	mkFile(t, chromeDir, "Profile 1", "History")

	firefoxDir := t.TempDir()
	mkFile(t, firefoxDir, "abc123.default-release", "logins.json")
	mkFile(t, firefoxDir, "abc123.default-release", "places.sqlite")

	tests := []struct {
		name         string
		configs      []types.BrowserConfig
		pickName     string
		profilePath  string
		wantNames    []string
		wantProfiles []string
	}{
		{
			name: "chromium uses path directly",
			configs: []types.BrowserConfig{
				{Key: "chrome", Name: "Chrome", Kind: types.KindChromium, UserDataDir: "/wrong"},
			},
			pickName:     "chrome",
			profilePath:  filepath.Join(chromeDir, "Default"),
			wantNames:    []string{"Chrome"},
			wantProfiles: []string{"Default"},
		},
		{
			name: "firefox uses parent dir",
			configs: []types.BrowserConfig{
				{Key: "firefox", Name: "Firefox", Kind: types.KindFirefox, UserDataDir: "/wrong"},
			},
			pickName:     "firefox",
			profilePath:  filepath.Join(firefoxDir, "abc123.default-release"),
			wantNames:    []string{"Firefox"},
			wantProfiles: []string{"abc123.default-release"},
		},
		{
			name: "ignored when name is all",
			configs: []types.BrowserConfig{
				{Key: "chrome", Name: "Chrome", Kind: types.KindChromium, UserDataDir: chromeDir},
			},
			pickName:     "all",
			profilePath:  "/some/override",
			wantNames:    []string{"Chrome", "Chrome"},
			wantProfiles: []string{"Default", "Profile 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := pickFromConfigs(tt.configs, tt.pickName, tt.profilePath)
			require.NoError(t, err)
			assertBrowsers(t, browsers, tt.wantNames, tt.wantProfiles)
		})
	}
}

// assertBrowsers verifies browser names and profiles match expectations (order-independent).
func assertBrowsers(t *testing.T, browsers []Browser, wantNames, wantProfiles []string) {
	t.Helper()
	assert.Len(t, browsers, len(wantNames))

	var gotNames, gotProfiles []string
	for _, b := range browsers {
		gotNames = append(gotNames, b.BrowserName())
		gotProfiles = append(gotProfiles, b.ProfileName())
	}
	sort.Strings(gotNames)
	sort.Strings(gotProfiles)
	sort.Strings(wantNames)
	sort.Strings(wantProfiles)

	assert.Equal(t, wantNames, gotNames)
	assert.Equal(t, wantProfiles, gotProfiles)
}
