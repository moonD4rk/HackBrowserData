package main

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

const goldenDir = "../../testdata"

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name)
	if *update {
		require.NoError(t, os.MkdirAll(goldenDir, 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got, "regenerate with: go test ./cmd/hack-browser-data -run '%s' -update", t.Name())
}

func bundleDir(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		require.Truef(t, e.Type().IsRegular(), "unexpected non-regular entry %s in %s", e.Name(), dir)
		names = append(names, e.Name())
	}
	return bundleFiles(t, dir, names...)
}

func bundleFiles(t *testing.T, dir string, names ...string) string {
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

func readDirFiles(t *testing.T, dir string) map[string]string {
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

// Cookie-Editor's own format covers cookies only; every other category is written exactly as the json format writes it.
func assertJSONFallback(t *testing.T, cookieEditorDir, jsonDir string) {
	t.Helper()
	got, want := readDirFiles(t, cookieEditorDir), readDirFiles(t, jsonDir)
	require.Contains(t, got, "cookie.json")
	delete(got, "cookie.json")
	delete(want, "cookie.json")
	assert.Equal(t, want, got)
}

func zipListing(t *testing.T, path string) string {
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
