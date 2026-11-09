package masterkey

import (
	"bytes"
	"testing"
)

func TestReadKeyFile_RejectsMissingVersion(t *testing.T) {
	input := bytes.NewBufferString(`{"created_at":"2026-05-16T00:00:00Z","host":{"os":"linux","arch":"amd64"},"vaults":[]}`)
	_, err := ReadKeyFile(input)
	if err == nil {
		t.Fatal("ReadKeyFile should reject empty version, got nil error")
	}
}

func TestReadKeyFile_AcceptsUnknownKeyVersions(t *testing.T) {
	input := bytes.NewBufferString(`{"version":"2","vaults":[{"browser":"chrome","keys":{"v10":"EA==","v99":"AQ=="}}]}`)
	keyFile, err := ReadKeyFile(input)
	if err != nil {
		t.Fatalf("ReadKeyFile: %v", err)
	}
	if got := keyFile.Vaults[0].Keys[V10]; !bytes.Equal(got, []byte{0x10}) {
		t.Errorf("v10 = %x, want 10", got)
	}
}
