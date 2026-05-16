package keyretriever

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"
	"time"
)

const DumpVersion = "1"

// Dump is the cross-host portable container for Chromium-family master keys.
// A Dump produced on one host can be consumed on another to skip platform-native
// key retrieval (DPAPI, ABE injection, Keychain prompt, D-Bus query) when
// decrypting a copy of the browser's profile data.
type Dump struct {
	Version       string         `json:"version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Host          DumpHost       `json:"host"`
	Installations []Installation `json:"installations"`
}

// DumpHost OS / Arch always set; Hostname / User best-effort (empty on syscall failure).
type DumpHost struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname,omitempty"`
	User     string `json:"user,omitempty"`
}

// Installation groups profiles sharing master keys (master keys are per-installation, not per-profile).
type Installation struct {
	Browser     string      `json:"browser"`
	UserDataDir string      `json:"user_data_dir"`
	Profiles    []string    `json:"profiles"`
	Keys        InstallKeys `json:"keys"`
}

// InstallKeys per-cipher-tier master keys; encoding/json marshals []byte as base64.
type InstallKeys struct {
	V10 []byte `json:"v10,omitempty"`
	V11 []byte `json:"v11,omitempty"`
	V20 []byte `json:"v20,omitempty"`
}

// NewDump returns a Dump initialized with current host metadata and an empty Installations slice
func NewDump() Dump {
	return Dump{
		Version:       DumpVersion,
		GeneratedAt:   time.Now().UTC(),
		Host:          currentHost(),
		Installations: []Installation{},
	}
}

// currentHost collects host identification. Hostname / User are best-effort:
// syscall failures leave the field empty, omitted from JSON via `omitempty`.
func currentHost() DumpHost {
	h := DumpHost{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if name, err := os.Hostname(); err == nil {
		h.Hostname = name
	}
	if u, err := user.Current(); err == nil {
		h.User = u.Username
	}
	return h
}

// WriteJSON writes the Dump as indented JSON to w.
func (d Dump) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return fmt.Errorf("encode dump: %w", err)
	}
	return nil
}

// ReadJSON parses a Dump from r.
func ReadJSON(r io.Reader) (Dump, error) {
	var d Dump
	dec := json.NewDecoder(r)
	if err := dec.Decode(&d); err != nil {
		return Dump{}, fmt.Errorf("decode dump: %w", err)
	}
	return d, nil
}
