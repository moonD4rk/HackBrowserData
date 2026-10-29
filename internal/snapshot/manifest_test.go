package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest_RoundTrip(t *testing.T) {
	d := Open(t.TempDir())
	want := Manifest{Version: ManifestVersion, Dirs: []ManifestDir{{
		ID: "arc-2", Browser: "arc", Base: "chromium",
		Profiles: []ManifestProfile{{Name: "Default", Dir: "Default"}},
		Files:    []string{"Local State", "Default/Preferences"},
	}}}
	require.NoError(t, d.WriteManifest(want))

	got, ok, err := d.ReadManifest()
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, want, got)
}

func TestManifest_AbsentIsNotAnError(t *testing.T) {
	_, ok, err := Open(t.TempDir()).ReadManifest()
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestManifest_RejectsAnotherVersion(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, manifestName), []byte(`{"version": 2, "dirs": []}`), 0o600))
	_, _, err := Open(root).ReadManifest()
	assert.Error(t, err)
}
