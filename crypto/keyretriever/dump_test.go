package keyretriever

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
