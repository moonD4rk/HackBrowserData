package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_ReplacesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	writeFile(t, path, "stale")

	require.NoError(t, Write(path, writeString("fresh")))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(got))
	assert.Equal(t, []string{"keys.json"}, filesIn(t, filepath.Dir(path)))
}

func TestWrite_FailureKeepsAnExistingFile(t *testing.T) {
	out := t.TempDir()
	path := filepath.Join(out, "keys.json")
	writeFile(t, path, "precious")

	failure := errors.New("encoder failed")
	err := Write(path, func(writer io.Writer) error {
		if _, err := io.WriteString(writer, "half"); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(got))
	assert.Equal(t, []string{"keys.json"}, filesIn(t, out), "no temp file may survive a failed write")
}

// TestWrite_IsOwnerOnly keeps a written file as private as the browser files and records it holds.
func TestWrite_IsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no mode bits to check")
	}
	path := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, Write(path, writeString("")))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWrite_LeavesAPlantedTempFileAlone(t *testing.T) {
	out := t.TempDir()
	path := filepath.Join(out, "out.zip")
	writeFile(t, path+".tmp", "planted")
	require.NoError(t, os.Chmod(path+".tmp", 0o644))

	require.NoError(t, Write(path, writeString("")))
	planted, err := os.ReadFile(path + ".tmp")
	require.NoError(t, err)
	assert.Equal(t, "planted", string(planted))
	assert.Equal(t, []string{"out.zip", "out.zip.tmp"}, filesIn(t, out))
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "a planted 0644 file must not set the file's mode")
	}
}

func TestWrite_DoesNotFollowAPlantedSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.zip")
	victim := filepath.Join(t.TempDir(), "victim")
	writeFile(t, victim, "precious")
	if err := os.Symlink(victim, path+".tmp"); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}

	require.NoError(t, Write(path, writeString("")))
	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(got))
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the written file must be a file, not the planted symlink")
}

func TestWrite_FailedRenameRemovesTheTempFile(t *testing.T) {
	out := t.TempDir()
	path := filepath.Join(out, "out.zip")
	require.NoError(t, os.Mkdir(path, 0o700))

	require.Error(t, Write(path, writeString("")))
	assert.Equal(t, []string{"out.zip"}, filesIn(t, out), "no temp file may survive a failed rename")
}

func writeString(content string) func(io.Writer) error {
	return func(writer io.Writer) error {
		_, err := io.WriteString(writer, content)
		return err
	}
}

func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	dirEntries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(dirEntries))
	for _, entry := range dirEntries {
		names = append(names, entry.Name())
	}
	return names
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}
