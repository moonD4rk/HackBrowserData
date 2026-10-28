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
	req, err := Validate(Flags{Browser: " Chrome ", Categories: "cookie, password,cookie", Format: "csv", Dir: "out", KeychainPassword: "pw"})
	require.NoError(t, err)
	assert.Equal(t, "chrome", req.Browser)
	assert.Equal(t, []category.Category{category.Cookie, category.Password}, req.Categories, "a repeated category runs once")
	assert.Equal(t, export.CSV, req.Format)
	assert.Equal(t, "out", req.Dir)
	assert.NotEmpty(t, req.Home)
	password, err := req.KeychainPassword(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pw", password)

	req, err = Validate(Flags{Categories: "all", Format: "json"})
	require.NoError(t, err)
	assert.Equal(t, "all", req.Browser)
	assert.Equal(t, category.All(), req.Categories)
}

func TestValidate_RejectsBadInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	missing := filepath.Join(t.TempDir(), "missing")
	for name, f := range map[string]Flags{
		"unknown category":     {Categories: "cookie,bogus", Format: "json"},
		"no category":          {Categories: " , ", Format: "json"},
		"unknown format":       {Categories: "all", Format: "xml"},
		"unknown browser":      {Browser: "bogus", Categories: "all", Format: "json"},
		"another OS's browser": {Browser: otherOSBrowser(t), Categories: "all", Format: "json"},
		"-d below a file":      {Categories: "all", Format: "json", Dir: filepath.Join(file, "out")},
		"-d is a file":         {Categories: "all", Format: "json", Dir: file},
		"missing -p":           {Browser: "chrome", Categories: "all", Format: "json", ProfilePath: missing},
		"-p is a file":         {Browser: "chrome", Categories: "all", Format: "json", ProfilePath: file},
		"two password inputs":  {Categories: "all", Format: "json", KeychainPassword: "pw", KeychainPasswordStdin: true},
	} {
		_, err := Validate(f)
		assert.Error(t, err, name)
	}
}

// otherOSBrowser is a browser in the table that the running OS does not have.
func otherOSBrowser(t *testing.T) string {
	t.Helper()
	for _, id := range []string{"sogou", "safari"} {
		def, ok := browser.Lookup(id)
		require.True(t, ok, id)
		if len(def.Platform().UserDataDirs) == 0 {
			return id
		}
	}
	t.Fatal("every candidate is supported here")
	return ""
}

func TestValidate_CreatesNoOutputDir(t *testing.T) {
	root := t.TempDir()
	req, err := Validate(Flags{Categories: "all", Format: "json", Dir: filepath.Join(root, "a", "b")})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "a", "b"), req.Dir)
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

	_, err := Validate(Flags{Categories: "all", Format: "json", Dir: filepath.Join(root, "results")})
	assert.Error(t, err)
}

// -p without a named browser was always ignored; it is still not checked, only reported.
func TestValidate_IgnoresProfilePathForAllBrowsers(t *testing.T) {
	req, err := Validate(Flags{Browser: "all", Categories: "all", Format: "json", ProfilePath: filepath.Join(t.TempDir(), "missing")})
	require.NoError(t, err)
	assert.Empty(t, req.ProfilePath)
}

func TestValidate_KeychainPasswordInputs(t *testing.T) {
	tests := []struct {
		name  string
		flags Flags
		stdin string
		want  string
		err   error
	}{
		{name: "flag", flags: Flags{KeychainPassword: "from-flag"}, stdin: "from-stdin\n", want: "from-flag"},
		{name: "stdin", flags: Flags{KeychainPasswordStdin: true}, stdin: "from-stdin\r\nnext\n", want: "from-stdin"},
		{name: "stdin without newline", flags: Flags{KeychainPasswordStdin: true}, stdin: "from-stdin", want: "from-stdin"},
		{name: "none off a terminal", flags: Flags{}, stdin: "from-stdin\n", err: errNoPassword},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			require.NoError(t, err)
			defer r.Close()
			_, err = w.WriteString(tt.stdin)
			require.NoError(t, errors.Join(err, w.Close()))

			tt.flags.Categories, tt.flags.Format = "all", "json"
			req, err := validate(tt.flags, r)
			require.NoError(t, err)
			got, err := req.KeychainPassword(context.Background())
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}
