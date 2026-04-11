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

type pickTest struct {
	name         string
	configs      []types.BrowserConfig
	opts         PickOptions
	wantNames    []string
	wantProfiles []string
}

func runPickTests(t *testing.T, tests []pickTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browsers, err := pickFromConfigs(tt.configs, tt.opts)
			require.NoError(t, err)
			assertBrowsers(t, browsers, tt.wantNames, tt.wantProfiles)
		})
	}
}

func TestPickFromConfigs(t *testing.T) {
	// --- fixtures: single-profile chromium (for name filter tests) ---
	singleDir := t.TempDir()
	mkFile(t, singleDir, "Default", "Preferences")
	mkFile(t, singleDir, "Default", "Login Data")
	mkFile(t, singleDir, "Default", "History")

	// --- fixtures: multi-profile chromium ---
	chromeDir := t.TempDir()
	mkFile(t, chromeDir, "Default", "Preferences")
	mkFile(t, chromeDir, "Default", "Login Data")
	mkFile(t, chromeDir, "Default", "History")
	mkFile(t, chromeDir, "Profile 1", "Preferences")
	mkFile(t, chromeDir, "Profile 1", "Login Data")
	mkFile(t, chromeDir, "Profile 1", "History")

	// --- fixtures: firefox ---
	firefoxDir := t.TempDir()
	mkFile(t, firefoxDir, "abc123.default-release", "logins.json")
	mkFile(t, firefoxDir, "abc123.default-release", "places.sqlite")

	// --- fixtures: yandex ---
	yandexDir := t.TempDir()
	mkFile(t, yandexDir, "Default", "Preferences")
	mkFile(t, yandexDir, "Default", "Ya Passman Data")
	mkFile(t, yandexDir, "Default", "History")

	// --- fixtures: glob (MSIX-like package directories) ---
	globBase := t.TempDir()
	mkFile(t, globBase, "App.Browser_abc123", "UserData", "Default", "Preferences")
	mkFile(t, globBase, "App.Browser_abc123", "UserData", "Default", "History")
	mkFile(t, globBase, "App.Browser_def456", "UserData", "Default", "Preferences")
	mkFile(t, globBase, "App.Browser_def456", "UserData", "Default", "History")
	mkFile(t, globBase, "Solo.Browser_xyz789", "UserData", "Default", "Preferences")
	mkFile(t, globBase, "Solo.Browser_xyz789", "UserData", "Default", "History")

	nameFilterConfigs := []types.BrowserConfig{
		{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: singleDir},
		{Key: "edge", Name: "Edge", Kind: types.Chromium, UserDataDir: singleDir},
	}

	t.Run("NameFilter", func(t *testing.T) {
		runPickTests(t, []pickTest{
			{
				name:         "exact match",
				configs:      nameFilterConfigs,
				opts:         PickOptions{Name: "chrome"},
				wantNames:    []string{"Chrome"},
				wantProfiles: []string{"Default"},
			},
			{
				name:         "case insensitive",
				configs:      nameFilterConfigs,
				opts:         PickOptions{Name: "Chrome"},
				wantNames:    []string{"Chrome"},
				wantProfiles: []string{"Default"},
			},
			{
				name:         "all returns both",
				configs:      nameFilterConfigs,
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Chrome", "Edge"},
				wantProfiles: []string{"Default", "Default"},
			},
			{
				name:    "unknown returns empty",
				configs: nameFilterConfigs,
				opts:    PickOptions{Name: "safari"},
			},
		})
	})

	t.Run("BrowserKind", func(t *testing.T) {
		runPickTests(t, []pickTest{
			{
				name: "chromium multi-profile",
				configs: []types.BrowserConfig{
					{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: chromeDir},
				},
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Chrome", "Chrome"},
				wantProfiles: []string{"Default", "Profile 1"},
			},
			{
				name: "firefox random dir",
				configs: []types.BrowserConfig{
					{Key: "firefox", Name: "Firefox", Kind: types.Firefox, UserDataDir: firefoxDir},
				},
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Firefox"},
				wantProfiles: []string{"abc123.default-release"},
			},
			{
				name: "yandex variant",
				configs: []types.BrowserConfig{
					{Key: "yandex", Name: "Yandex", Kind: types.ChromiumYandex, UserDataDir: yandexDir},
				},
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Yandex"},
				wantProfiles: []string{"Default"},
			},
			{
				name: "nonexistent dir",
				configs: []types.BrowserConfig{
					{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: "/nonexistent"},
				},
				opts: PickOptions{Name: "all"},
			},
		})
	})

	t.Run("ProfilePath", func(t *testing.T) {
		runPickTests(t, []pickTest{
			{
				name: "chromium uses path directly",
				configs: []types.BrowserConfig{
					{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: "/wrong"},
				},
				opts:         PickOptions{Name: "chrome", ProfilePath: filepath.Join(chromeDir, "Default")},
				wantNames:    []string{"Chrome"},
				wantProfiles: []string{"Default"},
			},
			{
				name: "firefox uses parent dir",
				configs: []types.BrowserConfig{
					{Key: "firefox", Name: "Firefox", Kind: types.Firefox, UserDataDir: "/wrong"},
				},
				opts:         PickOptions{Name: "firefox", ProfilePath: filepath.Join(firefoxDir, "abc123.default-release")},
				wantNames:    []string{"Firefox"},
				wantProfiles: []string{"abc123.default-release"},
			},
			{
				name: "ignored when name is all",
				configs: []types.BrowserConfig{
					{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: chromeDir},
				},
				opts:         PickOptions{Name: "all", ProfilePath: "/some/override"},
				wantNames:    []string{"Chrome", "Chrome"},
				wantProfiles: []string{"Default", "Profile 1"},
			},
		})
	})

	t.Run("Glob", func(t *testing.T) {
		runPickTests(t, []pickTest{
			{
				name: "single match",
				configs: []types.BrowserConfig{
					{Key: "solo", Name: "Solo", Kind: types.Chromium, UserDataDir: filepath.Join(globBase, "Solo.Browser_*", "UserData")},
				},
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Solo"},
				wantProfiles: []string{"Default"},
			},
			{
				name: "multiple matches",
				configs: []types.BrowserConfig{
					{Key: "arc", Name: "Arc", Kind: types.Chromium, UserDataDir: filepath.Join(globBase, "App.Browser_*", "UserData")},
				},
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Arc", "Arc"},
				wantProfiles: []string{"Default", "Default"},
			},
			{
				name: "no match",
				configs: []types.BrowserConfig{
					{Key: "missing", Name: "Missing", Kind: types.Chromium, UserDataDir: filepath.Join(globBase, "NoSuch_*", "UserData")},
				},
				opts: PickOptions{Name: "all"},
			},
			{
				name: "mixed with literal",
				configs: []types.BrowserConfig{
					{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: singleDir},
					{Key: "arc", Name: "Arc", Kind: types.Chromium, UserDataDir: filepath.Join(globBase, "Solo.Browser_*", "UserData")},
				},
				opts:         PickOptions{Name: "all"},
				wantNames:    []string{"Arc", "Chrome"},
				wantProfiles: []string{"Default", "Default"},
			},
			{
				name: "with name filter",
				configs: []types.BrowserConfig{
					{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: singleDir},
					{Key: "arc", Name: "Arc", Kind: types.Chromium, UserDataDir: filepath.Join(globBase, "App.Browser_*", "UserData")},
				},
				opts:         PickOptions{Name: "arc"},
				wantNames:    []string{"Arc", "Arc"},
				wantProfiles: []string{"Default", "Default"},
			},
		})
	})
}

func TestResolveGlobs(t *testing.T) {
	// Create directories for glob matching:
	//   base/
	//   ├── App.Browser_abc123/UserData/   (match 1)
	//   ├── App.Browser_def456/UserData/   (match 2)
	//   └── ExactBrowser/UserData/         (literal path)
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "App.Browser_abc123", "UserData"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "App.Browser_def456", "UserData"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "ExactBrowser", "UserData"), 0o755))

	tests := []struct {
		name     string
		configs  []types.BrowserConfig
		wantDirs []string // expected UserDataDir values after resolution
	}{
		{
			name: "literal path exists",
			configs: []types.BrowserConfig{
				{Key: "exact", UserDataDir: filepath.Join(base, "ExactBrowser", "UserData")},
			},
			wantDirs: []string{filepath.Join(base, "ExactBrowser", "UserData")},
		},
		{
			name: "literal path not exists preserved",
			configs: []types.BrowserConfig{
				{Key: "missing", UserDataDir: filepath.Join(base, "NoSuchBrowser", "UserData")},
			},
			wantDirs: []string{filepath.Join(base, "NoSuchBrowser", "UserData")},
		},
		{
			name: "glob single match",
			configs: []types.BrowserConfig{
				{Key: "single", UserDataDir: filepath.Join(base, "ExactBrow*", "UserData")},
			},
			wantDirs: []string{filepath.Join(base, "ExactBrowser", "UserData")},
		},
		{
			name: "glob multiple matches",
			configs: []types.BrowserConfig{
				{Key: "multi", UserDataDir: filepath.Join(base, "App.Browser_*", "UserData")},
			},
			wantDirs: []string{
				filepath.Join(base, "App.Browser_abc123", "UserData"),
				filepath.Join(base, "App.Browser_def456", "UserData"),
			},
		},
		{
			name: "glob no match preserved",
			configs: []types.BrowserConfig{
				{Key: "nomatch", UserDataDir: filepath.Join(base, "NoSuch_*", "UserData")},
			},
			wantDirs: []string{filepath.Join(base, "NoSuch_*", "UserData")},
		},
		{
			name: "mixed literal and glob",
			configs: []types.BrowserConfig{
				{Key: "chrome", UserDataDir: filepath.Join(base, "ExactBrowser", "UserData")},
				{Key: "arc", UserDataDir: filepath.Join(base, "App.Browser_*", "UserData")},
			},
			wantDirs: []string{
				filepath.Join(base, "ExactBrowser", "UserData"),
				filepath.Join(base, "App.Browser_abc123", "UserData"),
				filepath.Join(base, "App.Browser_def456", "UserData"),
			},
		},
		{
			name:     "empty input",
			configs:  nil,
			wantDirs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveGlobs(tt.configs)

			var gotDirs []string
			for _, cfg := range got {
				gotDirs = append(gotDirs, cfg.UserDataDir)
			}
			sort.Strings(gotDirs)
			sort.Strings(tt.wantDirs)
			assert.Equal(t, tt.wantDirs, gotDirs)

			// Verify non-UserDataDir fields are preserved.
			for _, cfg := range got {
				found := false
				for _, orig := range tt.configs {
					if cfg.Key != orig.Key {
						continue
					}
					found = true
					assert.Equal(t, orig.Name, cfg.Name)
					assert.Equal(t, orig.Kind, cfg.Kind)
					break
				}
				assert.True(t, found, "unexpected key %q in output", cfg.Key)
			}
		})
	}
}

func TestNewBrowsersDispatch(t *testing.T) {
	chromiumDir := t.TempDir()
	mkFile(t, chromiumDir, "Default", "Preferences")
	mkFile(t, chromiumDir, "Default", "History")

	firefoxDir := t.TempDir()
	mkFile(t, firefoxDir, "abc.default", "places.sqlite")

	safariDir := t.TempDir()
	mkFile(t, safariDir, "History.db")

	emptyDir := t.TempDir()

	tests := []struct {
		name        string
		cfg         types.BrowserConfig
		wantLen     int
		wantName    string
		wantProfile string
		wantErr     string
	}{
		{
			name:        "chromium dispatch",
			cfg:         types.BrowserConfig{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: chromiumDir},
			wantLen:     1,
			wantName:    "Chrome",
			wantProfile: "Default",
		},
		{
			name:        "firefox dispatch",
			cfg:         types.BrowserConfig{Key: "firefox", Name: "Firefox", Kind: types.Firefox, UserDataDir: firefoxDir},
			wantLen:     1,
			wantName:    "Firefox",
			wantProfile: "abc.default",
		},
		{
			name:        "safari dispatch",
			cfg:         types.BrowserConfig{Key: "safari", Name: "Safari", Kind: types.Safari, UserDataDir: safariDir},
			wantLen:     1,
			wantName:    "Safari",
			wantProfile: "default",
		},
		{
			name:    "unknown kind returns error",
			cfg:     types.BrowserConfig{Key: "unknown", Name: "Unknown", Kind: types.BrowserKind(99)},
			wantErr: "unknown browser kind",
		},
		{
			name: "empty dir returns empty",
			cfg:  types.BrowserConfig{Key: "chrome", Name: "Chrome", Kind: types.Chromium, UserDataDir: emptyDir},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := newBrowsers(tt.cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, found, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantName, found[0].BrowserName())
				assert.Equal(t, tt.wantProfile, found[0].ProfileName())
			}
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
