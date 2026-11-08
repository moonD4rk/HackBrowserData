package app

import (
	"context"
	"errors"
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

func TestValidate(t *testing.T) {
	options, err := ValidateDump(DumpFlags{Browser: " Chrome ", Categories: "cookie, password,cookie", Format: "csv", OutputDir: "out", Keychain: KeychainFlags{Password: "pw"}})
	require.NoError(t, err)
	assert.Equal(t, "chrome", options.Target.BrowserName)
	assert.Equal(t, []category.Category{category.Cookie, category.Password}, options.Categories, "a repeated category runs once")
	assert.Equal(t, export.CSV, options.Output.Format)
	assert.Equal(t, "out", options.Output.Dir)
	assert.NotEmpty(t, options.Target.HomeDir)
	assert.Equal(t, options.Target.HomeDir, options.Keychain.HomeDir)
	password, err := options.Keychain.Password(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pw", password)

	options, err = ValidateDump(DumpFlags{Categories: "all", Format: "json"})
	require.NoError(t, err)
	assert.Equal(t, "all", options.Target.BrowserName)
	assert.Equal(t, category.All(), options.Categories)
}

func TestValidate_RejectsBadInput(t *testing.T) {
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
		"two password inputs":  {Categories: "all", Format: "json", Keychain: KeychainFlags{Password: "pw", PasswordStdin: true}},
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
		if len(namedBrowser.Platform().UserDataDirs) == 0 {
			return name
		}
	}
	t.Fatal("every candidate is supported here")
	return ""
}

func TestValidate_CreatesNoOutputDir(t *testing.T) {
	root := t.TempDir()
	options, err := ValidateDump(DumpFlags{Categories: "all", Format: "json", OutputDir: filepath.Join(root, "a", "b")})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "a", "b"), options.Output.Dir)
	_, err = os.Stat(filepath.Join(root, "a"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestValidate_RejectsAnOutputDirThatCannotBeCreated(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny writes here")
	}
	root := t.TempDir()
	require.NoError(t, os.Chmod(root, 0o500))
	defer func() { require.NoError(t, os.Chmod(root, 0o700)) }()

	_, err := ValidateDump(DumpFlags{Categories: "all", Format: "json", OutputDir: filepath.Join(root, "results")})
	assert.Error(t, err)
}

// TestValidate_IgnoresProfilePathForAllBrowsers keeps -p without a named browser ignored, as before; it is reported.
func TestValidate_IgnoresProfilePathForAllBrowsers(t *testing.T) {
	options, err := ValidateDump(DumpFlags{Browser: "all", Categories: "all", Format: "json", ProfilePath: filepath.Join(t.TempDir(), "missing")})
	require.NoError(t, err)
	assert.Empty(t, options.Target.ProfilePath)
}

func TestValidate_KeychainPasswordInputs(t *testing.T) {
	tests := []struct {
		name  string
		flags DumpFlags
		stdin string
		want  string
		err   error
	}{
		{name: "flag", flags: DumpFlags{Keychain: KeychainFlags{Password: "from-flag"}}, stdin: "from-stdin\n", want: "from-flag"},
		{name: "stdin", flags: DumpFlags{Keychain: KeychainFlags{PasswordStdin: true}}, stdin: "from-stdin\r\nnext\n", want: "from-stdin"},
		{name: "stdin without newline", flags: DumpFlags{Keychain: KeychainFlags{PasswordStdin: true}}, stdin: "from-stdin", want: "from-stdin"},
		{name: "none off a terminal", flags: DumpFlags{}, stdin: "from-stdin\n", err: errNoPassword},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			pipeReader, pipeWriter, err := os.Pipe()
			require.NoError(t, err)
			defer pipeReader.Close()
			_, err = pipeWriter.WriteString(testCase.stdin)
			require.NoError(t, errors.Join(err, pipeWriter.Close()))

			testCase.flags.Categories, testCase.flags.Format = "all", "json"
			options, err := validateDump(testCase.flags, pipeReader)
			require.NoError(t, err)
			got, err := options.Keychain.Password(context.Background())
			require.ErrorIs(t, err, testCase.err)
			assert.Equal(t, testCase.want, got)
		})
	}
}
