package masterkey

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadJSON_RejectsUnknownVersion(t *testing.T) {
	input := bytes.NewBufferString(`{"version":"99","created_at":"2026-05-16T00:00:00Z","host":{"os":"linux","arch":"amd64"},"vaults":[]}`)
	_, err := ReadJSON(input)
	if err == nil {
		t.Fatal("ReadJSON should reject unknown version, got nil error")
	}
	if !strings.Contains(err.Error(), "unsupported dump version") {
		t.Errorf("error should mention unsupported version, got: %v", err)
	}
}

func TestReadJSON_RejectsMissingVersion(t *testing.T) {
	input := bytes.NewBufferString(`{"created_at":"2026-05-16T00:00:00Z","host":{"os":"linux","arch":"amd64"},"vaults":[]}`)
	_, err := ReadJSON(input)
	if err == nil {
		t.Fatal("ReadJSON should reject empty version, got nil error")
	}
}

func TestReadJSON_AcceptsCurrentVersion(t *testing.T) {
	d := NewDump()
	var buf bytes.Buffer
	if err := d.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	parsed, err := ReadJSON(&buf)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if parsed.Version != DumpVersion {
		t.Errorf("Version = %q, want %q", parsed.Version, DumpVersion)
	}
}

func TestDump_VaultKindRoundTrip(t *testing.T) {
	d := NewDump()
	d.Vaults = append(d.Vaults, Vault{
		Browser:     "chrome",
		Kind:        "chromium",
		UserDataDir: "/p",
		Profiles:    []string{"Default"},
		Keys:        MasterKeys{V10: []byte{0x01}},
	})

	var buf bytes.Buffer
	if err := d.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	parsed, err := ReadJSON(&buf)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
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
