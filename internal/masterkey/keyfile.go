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

const KeyFileVersion = "2"

// KeyFile is keys.json: produced on one host, it decrypts copied profile data on another.
type KeyFile struct {
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

// Vault names its on-disk format in Kind, so a consumer can read a vault without the browser in its own table.
type Vault struct {
	Browser string `json:"browser"`
	// ID names a second user data dir of the same browser ("arc-2"); the first one's is Browser, so it is left out.
	ID          string   `json:"id,omitempty"`
	Kind        string   `json:"kind"`
	UserDataDir string   `json:"user_data_dir"`
	Profiles    []string `json:"profiles"`
	Keys        Set      `json:"keys"`
}

func NewKeyFile() KeyFile {
	return KeyFile{
		Version:   KeyFileVersion,
		CreatedAt: time.Now().UTC(),
		Host:      currentHost(),
		Vaults:    []Vault{},
	}
}

func currentHost() Host {
	host := Host{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if name, err := os.Hostname(); err == nil {
		host.Hostname = name
	}
	if currentUser, err := user.Current(); err == nil {
		host.User = currentUser.Username
	}
	return host
}

func (d KeyFile) Write(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(d); err != nil {
		return fmt.Errorf("encode key file: %w", err)
	}
	return nil
}

// ReadKeyFile rejects a version it cannot interpret, since a silent misparse is worse than an error.
func ReadKeyFile(reader io.Reader) (KeyFile, error) {
	var dump KeyFile
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&dump); err != nil {
		return KeyFile{}, fmt.Errorf("decode key file: %w", err)
	}
	if dump.Version != KeyFileVersion {
		return KeyFile{}, fmt.Errorf("unsupported key file version %q (this build expects %q)", dump.Version, KeyFileVersion)
	}
	return dump, nil
}
