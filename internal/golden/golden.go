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

// updateEnv is an env var rather than a flag, so every package sees it and, read at test time, it keys the test cache.
const updateEnv = "HBD_UPDATE_GOLDEN"

const testdataDir = "testdata"

func Assert(t testing.TB, name, got string) {
	t.Helper()
	path := filepath.Join(testdataDir, name)
	if os.Getenv(updateEnv) == "1" {
		require.NoError(t, os.MkdirAll(testdataDir, 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got, "regenerate with: %s=1 go test ./... -run '%s'", updateEnv, t.Name())
}

func Read(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir, name))
	require.NoError(t, err)
	return string(data)
}

func ConcatDir(t testing.TB, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		require.Truef(t, entry.Type().IsRegular(), "unexpected non-regular entry %s in %s", entry.Name(), dir)
		names = append(names, entry.Name())
	}
	return ConcatFiles(t, dir, names...)
}

func ConcatFiles(t testing.TB, dir string, names ...string) string {
	t.Helper()
	var builder strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		fmt.Fprintf(&builder, "-- %s (%d bytes) --\n", name, len(data))
		builder.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func DirFiles(t testing.TB, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
		files[entry.Name()] = string(data)
	}
	return files
}

// AssertJSONFallback checks every file but cookie.json against the json output: Cookie-Editor formats only cookies.
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
	zipReader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer zipReader.Close()
	var builder strings.Builder
	for _, zipFile := range zipReader.File {
		fmt.Fprintf(&builder, "%s method=%d modified=%s size=%d crc32=%08x\n",
			zipFile.Name, zipFile.Method, zipFile.Modified.UTC().Format(time.RFC3339), zipFile.UncompressedSize64, zipFile.CRC32)
	}
	return builder.String()
}
