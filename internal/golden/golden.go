// Package golden compares test output with files in the calling package's testdata/; HBD_UPDATE_GOLDEN=1 rewrites them.
package golden

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An env var, not a test flag, reaches every package of go test ./... and, read at test time, keys its cache.
const updateEnv = "HBD_UPDATE_GOLDEN"

const dir = "testdata"

func Assert(t testing.TB, name, got string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if os.Getenv(updateEnv) == "1" {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got, "regenerate with: %s=1 go test ./... -run '%s'", updateEnv, t.Name())
}

func Read(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(data)
}

func Bundle(t testing.TB, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		require.Truef(t, e.Type().IsRegular(), "unexpected non-regular entry %s in %s", e.Name(), dir)
		names = append(names, e.Name())
	}
	return BundleFiles(t, dir, names...)
}

func BundleFiles(t testing.TB, dir string, names ...string) string {
	t.Helper()
	var b strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		fmt.Fprintf(&b, "-- %s (%d bytes) --\n", name, len(data))
		b.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func DirFiles(t testing.TB, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	files := make(map[string]string, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		files[e.Name()] = string(data)
	}
	return files
}

// Cookie-Editor writes its own format for cookies only, so every other file must match the json output.
func AssertJSONFallback(t testing.TB, cookieEditorDir, jsonDir string) {
	t.Helper()
	got, want := DirFiles(t, cookieEditorDir), DirFiles(t, jsonDir)
	require.Contains(t, got, "cookie.json")
	delete(got, "cookie.json")
	delete(want, "cookie.json")
	assert.Equal(t, want, got)
}

func ZipListing(t testing.TB, path string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer r.Close()
	var b strings.Builder
	for _, f := range r.File {
		fmt.Fprintf(&b, "%s method=%d modified=%s size=%d crc32=%08x\n",
			f.Name, f.Method, f.Modified.UTC().Format(time.RFC3339), f.UncompressedSize64, f.CRC32)
	}
	return b.String()
}
