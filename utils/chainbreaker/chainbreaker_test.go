package chainbreaker

import (
	"testing"
)

func TestUnlock(t *testing.T) {
	keychain, err := New("./testdata/test.keychain-db", "xxx")
	if err != nil {
		t.Fatalf("Failed to unlock keychain: %v", err)
	}
	records, err := keychain.DumpGenericPasswords()
	if err != nil {
		t.Fatal(err)
	}

	for _, rec := range records {
		t.Log("[+] Generic Password Record")
		t.Logf(" [-] Service: %s\n", rec.Service)
		t.Logf(" [-] Account: %s\n", rec.Account)
		t.Logf(" [-] Description: %s\n", rec.Description)
		t.Logf(" [-] Created: %s\n", rec.Created)
		t.Logf(" [-] Last Modified: %s\n", rec.LastModified)
		if rec.PasswordBase64 {
			t.Logf(" [-] Base64 Password: %s\n", rec.Password)
		} else {
			t.Logf(" [-] Password: %s\n", rec.Password)
		}
	}
}
