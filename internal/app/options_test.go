package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckOutputFile(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	require.NoError(t, checkOutputFile(filepath.Join(directory, "keys.json")))
	require.NoError(t, checkOutputFile(file), "an existing file is replaced")
	for name, path := range map[string]string{
		"missing directory":   filepath.Join(directory, "missing", "keys.json"),
		"directory is a file": filepath.Join(file, "keys.json"),
		"path is a directory": directory,
	} {
		assert.Error(t, checkOutputFile(path), name)
	}
}

// TestValidate_RejectsAnOutputFileItCannotWrite: -o fails before any browser is read, not after the work is done.
func TestValidate_RejectsAnOutputFileItCannotWrite(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing", "out")
	_, err := ValidateDumpKeys(DumpKeysFlags{Browser: "all", OutputPath: missing})
	require.Error(t, err)
	_, err = ValidateArchive(ArchiveFlags{Browser: "all", Categories: "all", OutputPath: missing})
	require.Error(t, err)

	_, err = ValidateDumpKeys(DumpKeysFlags{Browser: "all"})
	require.NoError(t, err, "without -o the key file goes to stdout")
}
