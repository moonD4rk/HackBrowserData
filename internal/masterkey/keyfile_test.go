package masterkey

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadKeyFile_RejectsUnknownVersion(t *testing.T) {
	input := bytes.NewBufferString(`{"version":"99","created_at":"2026-05-16T00:00:00Z","host":{"os":"linux","arch":"amd64"},"vaults":[]}`)
	_, err := ReadKeyFile(input)
	if err == nil {
		t.Fatal("ReadKeyFile should reject unknown version, got nil error")
	}
	if !strings.Contains(err.Error(), "unsupported key file version") {
		t.Errorf("error should mention unsupported version, got: %v", err)
	}
}

func TestReadKeyFile_RejectsMissingVersion(t *testing.T) {
	input := bytes.NewBufferString(`{"created_at":"2026-05-16T00:00:00Z","host":{"os":"linux","arch":"amd64"},"vaults":[]}`)
	_, err := ReadKeyFile(input)
	if err == nil {
		t.Fatal("ReadKeyFile should reject empty version, got nil error")
	}
}

func TestReadKeyFile_AcceptsCurrentVersion(t *testing.T) {
	keyFile := NewKeyFile()
	var buffer bytes.Buffer
	if err := keyFile.Write(&buffer); err != nil {
		t.Fatalf("Write: %v", err)
	}
	parsed, err := ReadKeyFile(&buffer)
	if err != nil {
		t.Fatalf("ReadKeyFile: %v", err)
	}
	if parsed.Version != KeyFileVersion {
		t.Errorf("Version = %q, want %q", parsed.Version, KeyFileVersion)
	}
}

func TestKeyFile_VaultKindRoundTrip(t *testing.T) {
	keyFile := NewKeyFile()
	keyFile.Vaults = append(keyFile.Vaults, Vault{
		Browser:     "chrome",
		Kind:        "chromium",
		UserDataDir: "/p",
		Profiles:    []string{"Default"},
		Keys:        Set{V10: []byte{0x01}},
	})

	var buffer bytes.Buffer
	if err := keyFile.Write(&buffer); err != nil {
		t.Fatalf("Write: %v", err)
	}
	parsed, err := ReadKeyFile(&buffer)
	if err != nil {
		t.Fatalf("ReadKeyFile: %v", err)
	}
	if len(parsed.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1", len(parsed.Vaults))
	}
	if parsed.Vaults[0].Kind != "chromium" {
		t.Errorf("Vault.Kind round-trip: got %q, want %q", parsed.Vaults[0].Kind, "chromium")
	}
	if parsed.Vaults[0].Browser != "chrome" {
		t.Errorf("Vault.Browser round-trip: got %q, want %q", parsed.Vaults[0].Browser, "chrome")
	}
}

// TestKeyFile_KeysWireForm pins the frozen keys.json form: present versions only, in key order, {} for no keys.
func TestKeyFile_KeysWireForm(t *testing.T) {
	keyFile := KeyFile{Version: KeyFileVersion, Vaults: []Vault{
		{Browser: "chrome", Keys: Set{V20: []byte{0x20}, V10: []byte{0x10}}},
		{Browser: "opera", Keys: Set{}},
	}}
	var buffer bytes.Buffer
	if err := keyFile.Write(&buffer); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buffer.String()
	for _, want := range []string{`"keys": {` + "\n" + `        "v10": "EA==",` + "\n" + `        "v20": "IA=="`, `"keys": {}`} {
		if !strings.Contains(out, want) {
			t.Errorf("Write output lacks %q:\n%s", want, out)
		}
	}
}

func TestReadKeyFile_IgnoresUnknownKeyVersions(t *testing.T) {
	input := bytes.NewBufferString(`{"version":"2","vaults":[{"browser":"chrome","keys":{"v10":"EA==","v99":"AQ=="}}]}`)
	keyFile, err := ReadKeyFile(input)
	if err != nil {
		t.Fatalf("ReadKeyFile: %v", err)
	}
	if got := keyFile.Vaults[0].Keys[V10]; !bytes.Equal(got, []byte{0x10}) {
		t.Errorf("v10 = %x, want 10", got)
	}
}
