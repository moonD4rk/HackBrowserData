package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
)

func TestValidateDump(t *testing.T) {
	keychainPassword := func(context.Context) (string, error) { return "pw", nil }
	options, err := ValidateDump(DumpFlags{Browser: " Chrome ", Categories: "cookie, password,cookie", Format: "csv", OutputDir: "out"}, keychainPassword)
	require.NoError(t, err)
	require.Len(t, options.Target.Browsers, 1)
	assert.Equal(t, "chrome", options.Target.Browsers[0].Name)
	assert.Equal(t, []category.Category{category.Cookie, category.Password}, options.Categories, "a repeated category runs once")
	assert.Equal(t, export.CSV, options.Output.Format)
	assert.Equal(t, "out", options.Output.Dir)
	assert.NotEmpty(t, options.Target.HomeDir)
	assert.Equal(t, loginKeychainPath(options.Target.HomeDir), options.Keychain.Path)
	password, err := options.Keychain.Password(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pw", password)

	options, err = ValidateDump(DumpFlags{Categories: "all", Format: "json"}, nil)
	require.NoError(t, err)
	assert.Equal(t, browser.Supported(), options.Target.Browsers)
	assert.Equal(t, category.All(), options.Categories)
}

func TestValidateDump_RejectsBadInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	missing := filepath.Join(t.TempDir(), "missing")
	for name, flags := range map[string]DumpFlags{
		"unknown category":     {Categories: "cookie,bogus", Format: "json"},
		"no category":          {Categories: " , ", Format: "json"},
		"unknown format":       {Categories: "all", Format: "xml"},
		"unknown browser":      {Browser: "bogus", Categories: "all", Format: "json"},
		"another OS's browser": {Browser: otherOSBrowser(t), Categories: "all", Format: "json"},
		"-d below a file":      {Categories: "all", Format: "json", OutputDir: filepath.Join(file, "out")},
		"-d is a file":         {Categories: "all", Format: "json", OutputDir: file},
		"missing -p":           {Browser: "chrome", Categories: "all", Format: "json", ProfilePath: missing},
		"-p is a file":         {Browser: "chrome", Categories: "all", Format: "json", ProfilePath: file},
	} {
		_, err := ValidateDump(flags, nil)
		assert.Error(t, err, name)
	}
}

func otherOSBrowser(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"sogou", "safari"} {
		namedBrowser, ok := browser.Lookup(name)
		require.True(t, ok, name)
		if len(namedBrowser.Platform().UserDataDirs) == 0 {
			return name
		}
	}
	t.Fatal("every candidate is supported here")
	return ""
}

func TestValidateDump_CreatesNoOutputDir(t *testing.T) {
	root := t.TempDir()
	options, err := ValidateDump(DumpFlags{Categories: "all", Format: "json", OutputDir: filepath.Join(root, "a", "b")}, nil)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "a", "b"), options.Output.Dir)
	_, err = os.Stat(filepath.Join(root, "a"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestValidateDump_RejectsAnOutputDirThatCannotBeCreated(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny writes here")
	}
	root := t.TempDir()
	require.NoError(t, os.Chmod(root, 0o500))
	defer func() { require.NoError(t, os.Chmod(root, 0o700)) }()

	_, err := ValidateDump(DumpFlags{Categories: "all", Format: "json", OutputDir: filepath.Join(root, "results")}, nil)
	assert.Error(t, err)
}

// TestValidateDump_IgnoresProfilePathForAllBrowsers keeps -p without a named browser ignored, as before; it is reported.
func TestValidateDump_IgnoresProfilePathForAllBrowsers(t *testing.T) {
	options, err := ValidateDump(DumpFlags{Browser: "all", Categories: "all", Format: "json", ProfilePath: filepath.Join(t.TempDir(), "missing")}, nil)
	require.NoError(t, err)
	assert.Empty(t, options.Target.ProfilePath)
}
