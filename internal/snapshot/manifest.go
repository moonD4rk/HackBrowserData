package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const ManifestVersion = 1

const manifestName = "manifest.json"

// Manifest records what a snapshot directory holds, so a reader rebuilds its user data dirs without discovering them.
type Manifest struct {
	Version int           `json:"version"`
	Dirs    []ManifestDir `json:"dirs"`
}

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

func (d *Dir) WriteManifest(manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
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
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, false, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.Version != ManifestVersion {
		return Manifest{}, false, fmt.Errorf("unsupported manifest version %d (this build reads %d)", manifest.Version, ManifestVersion)
	}
	return manifest, true, nil
}
