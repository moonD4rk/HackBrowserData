package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ManifestVersion is the manifest.json schema this build writes and reads.
const ManifestVersion = 1

const manifestName = "manifest.json"

// Manifest records what a snapshot directory holds, so a reader rebuilds its user data dirs without discovering them.
type Manifest struct {
	Version int           `json:"version"`
	Dirs    []ManifestDir `json:"dirs"`
}

// ManifestDir is one user data dir in the snapshot directory: its DirID, the browser it belongs to, and what was
// captured from it.
type ManifestDir struct {
	ID       string            `json:"id"`
	Browser  string            `json:"browser"`
	Base     string            `json:"base"`
	Profiles []ManifestProfile `json:"profiles"`
	Files    []string          `json:"files"`
}

type ManifestProfile struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

func (d *Dir) WriteManifest(m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d.path, manifestName), append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// ReadManifest reports false for a snapshot directory without a manifest, such as an archive written before it.
func (d *Dir) ReadManifest() (Manifest, bool, error) {
	data, err := os.ReadFile(filepath.Join(d.path, manifestName))
	if errors.Is(err, fs.ErrNotExist) {
		return Manifest{}, false, nil
	}
	if err != nil {
		return Manifest{}, false, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, false, fmt.Errorf("decode manifest: %w", err)
	}
	if m.Version != ManifestVersion {
		return Manifest{}, false, fmt.Errorf("unsupported manifest version %d (this build reads %d)", m.Version, ManifestVersion)
	}
	return m, true, nil
}
