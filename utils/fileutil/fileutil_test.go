package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDir(t *testing.T, files []string) string {
	t.Helper() // Marks the function as a helper function.

	tempDir, err := os.MkdirTemp("", "testCompressDir")
	require.NoError(t, err, "failed to create a temporary directory")

	for _, file := range files {
		filePath := filepath.Join(tempDir, file)
		err := os.WriteFile(filePath, []byte("test content"), 0o644)
		require.NoError(t, err, "failed to create a test file")
	}
	return tempDir
}

func TestCompressDir(t *testing.T) {
	t.Run("Normal Operation", func(t *testing.T) {
		tempDir := setupTestDir(t, []string{"file1.txt", "file2.txt", "file3.txt"})
		defer os.RemoveAll(tempDir)

		err := CompressDir(tempDir)
		assert.NoError(t, err, "compressDir should not return an error")

		// Check if the zip file exists
		zipFile := filepath.Join(tempDir, filepath.Base(tempDir)+".zip")
		assert.FileExists(t, zipFile, "zip file should be created")
	})

	t.Run("Directory Does Not Exist", func(t *testing.T) {
		err := CompressDir("/path/to/nonexistent/directory")
		assert.Error(t, err, "should return an error for non-existent directory")
	})

	t.Run("Empty Directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "testEmptyDir")
		require.NoError(t, err, "failed to create empty test directory")
		defer os.RemoveAll(tempDir)

		err = CompressDir(tempDir)
		assert.Error(t, err, "should return an error for an empty directory")
	})
}
