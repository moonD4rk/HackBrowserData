package browsertest

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/zipfile"
)

// ZipDir packs every file under dir into archivePath, the way an archive carries a restore's data.
func ZipDir(t *testing.T, dir, archivePath string) {
	t.Helper()
	var names []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relativePath, err := filepath.Rel(dir, path)
		names = append(names, filepath.ToSlash(relativePath))
		return err
	}))
	require.NoError(t, zipfile.Write(archivePath, dir, names))
}
