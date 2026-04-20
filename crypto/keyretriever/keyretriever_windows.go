//go:build windows

package keyretriever

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

// DPAPIRetriever reads the encrypted key from Chrome's Local State file
// and decrypts it using Windows DPAPI.
type DPAPIRetriever struct{}

func (r *DPAPIRetriever) RetrieveKey(_, localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("read Local State: %w", err)
	}

	encryptedKey := gjson.GetBytes(data, "os_crypt.encrypted_key")
	if !encryptedKey.Exists() {
		return nil, fmt.Errorf("os_crypt.encrypted_key not found in Local State")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(encryptedKey.String())
	if err != nil {
		return nil, fmt.Errorf("base64 decode encrypted_key: %w", err)
	}

	// First 5 bytes are the "DPAPI" prefix, validate and skip them
	const dpapiPrefix = "DPAPI"
	if len(keyBytes) <= len(dpapiPrefix) {
		return nil, fmt.Errorf("encrypted_key too short: %d bytes", len(keyBytes))
	}
	if string(keyBytes[:len(dpapiPrefix)]) != dpapiPrefix {
		return nil, fmt.Errorf("encrypted_key unexpected prefix: got %q, want %q", keyBytes[:len(dpapiPrefix)], dpapiPrefix)
	}

	masterKey, err := crypto.DecryptDPAPI(keyBytes[len(dpapiPrefix):])
	if err != nil {
		return nil, fmt.Errorf("DPAPI decrypt: %w", err)
	}
	return masterKey, nil
}

// DefaultRetrievers returns the Windows Retrievers: DPAPI for v10 (Chrome's os_crypt.encrypted_key)
// and ABE for v20 (Chrome 127+ os_crypt.app_bound_encrypted_key retrieved via reflective injection
// into the browser's elevation service). Both run independently — a single Chrome profile upgraded
// from pre-v127 carries mixed v10+v20 ciphertexts, and both tiers must be attempted to decrypt the
// full profile (see issue #578).
func DefaultRetrievers() Retrievers {
	return Retrievers{
		V10: &DPAPIRetriever{},
		V20: &ABERetriever{},
	}
}
