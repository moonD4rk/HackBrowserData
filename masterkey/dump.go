package masterkey

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"
	"time"
)

const DumpVersion = "2"

// Dump is the portable, cross-host container for Chromium master keys — produce it on one host to
// decrypt copied profile data on another without DPAPI / ABE / Keychain / D-Bus.
type Dump struct {
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Host      Host      `json:"host"`
	Vaults    []Vault   `json:"vaults"`
}

// Host OS / Arch always set; Hostname / User best-effort (empty on syscall failure).
type Host struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname,omitempty"`
	User     string `json:"user,omitempty"`
}

// Vault groups profiles sharing master keys (master keys are per-installation, not per-profile).
// Browser is the lookup key (e.g. "chrome"); Kind is the engine ("chromium"|"chromium-yandex"|
// "chromium-opera") so a consumer can rebuild the engine without the local browser table.
type Vault struct {
	Browser     string     `json:"browser"`
	Kind        string     `json:"kind"`
	UserDataDir string     `json:"user_data_dir"`
	Profiles    []string   `json:"profiles"`
	Keys        MasterKeys `json:"keys"`
}

func NewDump() Dump {
	return Dump{
		Version:   DumpVersion,
		CreatedAt: time.Now().UTC(),
		Host:      currentHost(),
		Vaults:    []Vault{},
	}
}

func currentHost() Host {
	h := Host{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if name, err := os.Hostname(); err == nil {
		h.Hostname = name
	}
	if u, err := user.Current(); err == nil {
		h.User = u.Username
	}
	return h
}

func (d Dump) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return fmt.Errorf("encode dump: %w", err)
	}
	return nil
}

// ReadJSON parses a Dump and rejects any version this build can't interpret — a silent misparse of an
// unrecognized schema is worse than a clear error.
func ReadJSON(r io.Reader) (Dump, error) {
	var d Dump
	dec := json.NewDecoder(r)
	if err := dec.Decode(&d); err != nil {
		return Dump{}, fmt.Errorf("decode dump: %w", err)
	}
	if d.Version != DumpVersion {
		return Dump{}, fmt.Errorf("unsupported dump version %q (this build expects %q)", d.Version, DumpVersion)
	}
	return d, nil
}
