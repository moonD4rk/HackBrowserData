package filemanager

import (
	"os"
	"strings"

	cp "github.com/otiai10/copy"
)

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

// copyDir copies a directory from src to dst, skipping files
// whose path ends with the skip suffix (e.g. "lock").
func copyDir(src, dst, skip string) error {
	opts := cp.Options{Skip: func(info os.FileInfo, src, _ string) (bool, error) {
		return strings.HasSuffix(strings.ToLower(src), skip), nil
	}}
	return cp.Copy(src, dst, opts)
}

// isFileExists checks if a file (not directory) exists at the given path.
func isFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
