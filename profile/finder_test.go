package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/types2"
)

func TestNewManager_ChromiumMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping test on non-darwin system")
	}
	paths, err := filepath.Glob(`/Users/*/Library/Application Support/Google/Chrome`)
	assert.NoError(t, err)
	if len(paths) == 0 {
		t.Skip("no chrome profile found")
	}
	rootPath := paths[0]
	browserType := types2.ChromiumType
	dataTypes := types2.AllDataTypes
	finder := NewFinder()
	profiles, err := finder.FindProfiles(rootPath, browserType, dataTypes)
	assert.NoError(t, err)
	assert.NotNil(t, profiles)
	for name, profile := range profiles {
		for k, v := range profile.DataFilePath {
			t.Logf("name: %s, datatype: %s, datapath: %s", name, k.String(), v)
		}
		t.Log(name, profile.MasterKeyPath)
	}
}

func TestProfileFinder_FirefoxMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping test on non-darwin system")
	}
	paths, err := filepath.Glob(`/Users/*/Library/Application Support/Firefox/Profiles`)
	assert.NoError(t, err)
	if len(paths) == 0 {
		t.Skip("no firefox profile found")
	}
	rootPath := paths[0]
	browserType := types2.FirefoxType
	dataTypes := types2.AllDataTypes
	finder := NewFinder()
	profiles, err := finder.FindProfiles(rootPath, browserType, dataTypes)
	assert.NoError(t, err)
	assert.NotNil(t, profiles)
	for name, profile := range profiles {
		for k, v := range profile.DataFilePath {
			t.Logf("name: %s, datatype: %s, value: %s", name, k.String(), v)
		}
		t.Log(name, profile.MasterKeyPath)
	}
}

func TestNewManager_ChromiumWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping test on non-windows system")
	}
	userProfile := os.Getenv("USERPROFILE")
	rootPath := filepath.Join(userProfile, `AppData\Local\Google\Chrome\User Data`)
	paths, err := filepath.Glob(rootPath)

	assert.NoError(t, err)
	if len(paths) == 0 {
		t.Skip("no chrome profile found")
	}
	browserType := types2.ChromiumType
	dataTypes := types2.AllDataTypes
	finder := NewFinder()
	profiles, err := finder.FindProfiles(rootPath, browserType, dataTypes)
	assert.NoError(t, err)
	assert.NotNil(t, profiles)
	for name, profile := range profiles {
		for k, v := range profile.DataFilePath {
			t.Logf("name: %s, datatype: %s, datapath: %s", name, k.String(), v)
		}
		t.Log(name, profile.MasterKeyPath)
	}
}

func TestProfileFinder_FirefoxWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping test on non-windows system")
	}
	userProfile := os.Getenv("USERPROFILE")
	rootPath := filepath.Join(userProfile, `AppData\Roaming\Mozilla\Firefox\Profiles`)
	paths, err := filepath.Glob(rootPath)

	assert.NoError(t, err)
	if len(paths) == 0 {
		t.Skip("no firefox profile found")
	}
	browserType := types2.FirefoxType
	dataTypes := types2.AllDataTypes
	finder := NewFinder()
	profiles, err := finder.FindProfiles(rootPath, browserType, dataTypes)
	assert.NoError(t, err)
	assert.NotNil(t, profiles)
	for name, profile := range profiles {
		for k, v := range profile.DataFilePath {
			t.Logf("name: %s, datatype: %s, value: %s", name, k.String(), v)
		}
		t.Log(name, profile.MasterKeyPath)
	}
}

func Test_extractProfileNameFromPath(t *testing.T) {
	testCases := []struct {
		name             string
		basePath         string
		currentPath      string
		isCurrentPathDir bool
		expected         string
	}{
		{
			name:             "Valid profile with data file",
			basePath:         filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles"),
			currentPath:      filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles/abcd1234.default-release/cookies.sqlite"),
			isCurrentPathDir: false,
			expected:         "abcd1234.default-release",
		},
		{
			name:             "Valid profile without data file",
			basePath:         filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles"),
			currentPath:      filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles/abcd1234.default-release/"),
			isCurrentPathDir: true,
			expected:         "abcd1234.default-release",
		},
		{
			name:             "Invalid path outside basePath",
			basePath:         filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles"),
			currentPath:      filepath.FromSlash("/Users/username/Documents/cookies.sqlite"),
			isCurrentPathDir: false,
			expected:         "",
		},
		{
			name:             "MasterKey in root directory",
			basePath:         filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles"),
			currentPath:      filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles/key4.db"),
			isCurrentPathDir: false,
			expected:         "",
		},
		{
			name:             "Nested profile directory",
			basePath:         filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles"),
			currentPath:      filepath.FromSlash("/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles/abcd1234.default-release/subdir/cookies.sqlite"),
			isCurrentPathDir: false,
			expected:         "abcd1234.default-release",
		},
		{
			name:        "Windows path format",
			basePath:    filepath.FromSlash("C:/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles"),
			currentPath: filepath.FromSlash("C:/Users/username/AppData/Roaming/Mozilla/Firefox/Profiles/abcd1234.default-release/cookies.sqlite"),
			expected:    "abcd1234.default-release",
		},
		{
			name:             "cookies in network path",
			basePath:         filepath.FromSlash("C:/AppData/Local/Google/Chrome/User Data"),
			currentPath:      filepath.FromSlash("C:/AppData/Local/Google/Chrome/User Data/Profile 1/Network/Cookies"),
			isCurrentPathDir: false,
			expected:         "Profile 1",
		},
		{
			name:             "cookies in network path",
			basePath:         filepath.FromSlash("C:/AppData/Local/Google/Chrome/User Data"),
			currentPath:      filepath.FromSlash("C:/AppData/Local/Google/Chrome/User Data"),
			isCurrentPathDir: true,
			expected:         "",
		},
		{
			name:             "local state",
			basePath:         filepath.FromSlash("/Users/moond4rk/Library/Application Support/Google/Chrome/"),
			currentPath:      filepath.FromSlash("/Users/moond4rk/Library/Application Support/Google/Chrome/Local State"),
			isCurrentPathDir: false,
			expected:         "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profileName := extractProfileName(tc.basePath, tc.currentPath, tc.isCurrentPathDir)
			if profileName != tc.expected {
				t.Errorf("extractProfileName(%q, %q) = %q; expected %q", tc.basePath, tc.currentPath, profileName, tc.expected)
			}
		})
	}
}
