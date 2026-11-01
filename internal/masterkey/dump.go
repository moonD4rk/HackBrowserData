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

// Dump is keys.json: produced on one host, it decrypts copied profile data on another.
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

// Kind names the engine, so a consumer can read a vault without the browser in its own table.
type Vault struct {
	Browser string `json:"browser"`
	// ID names a second user data dir of the same browser ("arc-2"); the first one's is Browser, so it is left out.
	ID          string   `json:"id,omitempty"`
	Kind        string   `json:"kind"`
	UserDataDir string   `json:"user_data_dir"`
	Profiles    []string `json:"profiles"`
	Keys        Set      `json:"keys"`
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

// ReadJSON rejects a version it cannot interpret, since a silent misparse is worse than an error.
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
