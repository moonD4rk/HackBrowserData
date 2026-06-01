//go:build windows

package masterkey

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

// DPAPIRetriever unwraps Chrome's Local State os_crypt.encrypted_key via Windows DPAPI.
type DPAPIRetriever struct{}

func (r *DPAPIRetriever) RetrieveKey(hints Hints) ([]byte, error) {
	data, err := os.ReadFile(hints.LocalStatePath)
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

// DefaultRetrievers wires the Windows tiers: DPAPI for v10, ABE for v20 (Chrome 127+, via reflective
// injection). Both run — a profile upgraded from pre-v127 mixes v10+v20 and needs both (issue #578).
func DefaultRetrievers() Retrievers {
	return Retrievers{
		V10: &DPAPIRetriever{},
		V20: &ABERetriever{},
	}
}
