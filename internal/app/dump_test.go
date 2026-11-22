package app

import (
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
	options, err := ValidateDump(DumpFlags{Browser: " Chrome ", Categories: "cookie, password,cookie", Output: OutputFlags{Format: "csv", Dir: "out"}})
	require.NoError(t, err)
	require.Len(t, options.Browsers, 1)
	assert.Equal(t, "chrome", options.Browsers[0].Name)
	assert.Equal(t, []category.Category{category.Cookie, category.Password}, options.Categories, "a repeated category runs once")
	assert.Equal(t, export.CSV, options.Output.Format)
	assert.Equal(t, "out", options.Output.Dir)
	assert.NotEmpty(t, options.HomeDir)

	options, err = ValidateDump(DumpFlags{Categories: "all", Output: OutputFlags{Format: "json"}})
	require.NoError(t, err)
	assert.Equal(t, browser.Supported(), options.Browsers)
	assert.Equal(t, category.All(), options.Categories)
}

func TestValidateDump_RejectsBadInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	missing := filepath.Join(t.TempDir(), "missing")
	for name, flags := range map[string]DumpFlags{
		"unknown category":     {Categories: "cookie,bogus", Output: OutputFlags{Format: "json"}},
		"no category":          {Categories: " , ", Output: OutputFlags{Format: "json"}},
		"unknown format":       {Categories: "all", Output: OutputFlags{Format: "xml"}},
		"unknown browser":      {Browser: "bogus", Categories: "all", Output: OutputFlags{Format: "json"}},
		"another OS's browser": {Browser: otherOSBrowser(t), Categories: "all", Output: OutputFlags{Format: "json"}},
		"-d below a file":      {Categories: "all", Output: OutputFlags{Format: "json", Dir: filepath.Join(file, "out")}},
		"-d is a file":         {Categories: "all", Output: OutputFlags{Format: "json", Dir: file}},
		"missing -p":           {Browser: "chrome", Categories: "all", Output: OutputFlags{Format: "json"}, ProfilePath: missing},
		"-p is a file":         {Browser: "chrome", Categories: "all", Output: OutputFlags{Format: "json"}, ProfilePath: file},
	} {
		_, err := ValidateDump(flags)
		assert.Error(t, err, name)
	}
}

func otherOSBrowser(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"sogou", "safari"} {
		namedBrowser, ok := browser.Lookup(name)
		require.True(t, ok, name)
		if len(namedBrowser.Platform().UserDataDirPaths) == 0 {
			return name
		}
	}
	t.Fatal("every candidate is supported here")
	return ""
}

func TestValidateDump_CreatesNoOutputDir(t *testing.T) {
	root := t.TempDir()
	options, err := ValidateDump(DumpFlags{Categories: "all", Output: OutputFlags{Format: "json", Dir: filepath.Join(root, "a", "b")}})
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

	_, err := ValidateDump(DumpFlags{Categories: "all", Output: OutputFlags{Format: "json", Dir: filepath.Join(root, "results")}})
	assert.Error(t, err)
}

// TestValidateDump_IgnoresProfilePathForAllBrowsers keeps -p without a named browser ignored, as before; it is reported.
func TestValidateDump_IgnoresProfilePathForAllBrowsers(t *testing.T) {
	options, err := ValidateDump(DumpFlags{Browser: "all", Categories: "all", Output: OutputFlags{Format: "json"}, ProfilePath: filepath.Join(t.TempDir(), "missing")})
	require.NoError(t, err)
	assert.Empty(t, options.ProfilePath)
}
