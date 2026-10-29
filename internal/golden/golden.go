// Package golden compares test output against files in the calling package's testdata/, which -update rewrites.
package golden

import (
	"archive/zip"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current code")

const dir = "testdata"

// Assert compares got with testdata/<name>, rewriting it first when -update is set.
func Assert(t testing.TB, name, got string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if *update {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got, "regenerate with: go test ./... -run '%s' -update", t.Name())
}

// Read returns testdata/<name>.
func Read(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(data)
}

// Bundle renders every regular file in dir as a named, length-tagged block, in directory order.
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

// BundleFiles renders the named files under dir as named, length-tagged blocks, in the given order.
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

// DirFiles maps each file name under dir to its contents.
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

// AssertJSONFallback checks that Cookie-Editor output matches the json output for every category but cookies, whose
// own format Cookie-Editor alone writes.
func AssertJSONFallback(t testing.TB, cookieEditorDir, jsonDir string) {
	t.Helper()
	got, want := DirFiles(t, cookieEditorDir), DirFiles(t, jsonDir)
	require.Contains(t, got, "cookie.json")
	delete(got, "cookie.json")
	delete(want, "cookie.json")
	assert.Equal(t, want, got)
}

// ZipListing renders a zip's entries as one deterministic line each: name, method, modified time, size, and CRC.
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
