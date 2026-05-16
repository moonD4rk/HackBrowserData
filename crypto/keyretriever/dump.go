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

// Dump is the cross-host portable container for Chromium master keys. Producing it on one host lets another host skip
// platform-native retrieval (DPAPI, ABE injection, Keychain prompt, D-Bus query) when decrypting copied profile data.
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
type Vault struct {
	Browser     string     `json:"browser"`
	UserDataDir string     `json:"user_data_dir"`
	Profiles    []string   `json:"profiles"`
	Keys        MasterKeys `json:"keys"`
}

// NewDump returns a Dump initialized with current host metadata and an empty Vaults slice
func NewDump() Dump {
	return Dump{
		Version:   DumpVersion,
		CreatedAt: time.Now().UTC(),
		Host:      currentHost(),
		Vaults:    []Vault{},
	}
}

// currentHost collects host identification; Hostname/User are best-effort (syscall failure leaves them empty + omitempty).
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
