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

	// First 5 bytes are the "DPAPI" prefix, skip them
	if len(keyBytes) <= 5 {
		return nil, fmt.Errorf("encrypted_key too short: %d bytes", len(keyBytes))
	}

	masterKey, err := crypto.DecryptWithDPAPI(keyBytes[5:])
	if err != nil {
		return nil, fmt.Errorf("DPAPI decrypt: %w", err)
	}
	return masterKey, nil
}

// DefaultRetriever returns the Windows retriever (DPAPI only).
func DefaultRetriever() KeyRetriever {
	return &DPAPIRetriever{}
}
