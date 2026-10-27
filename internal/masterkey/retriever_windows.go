//go:build windows

package masterkey

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/winapi"
)

// Both run: a profile upgraded past Chrome 127 holds v10 and v20 values side by side.
var platformRetrievers = []Retriever{dpapiRetriever{}, abeRetriever{}}

// dpapiRetriever unwraps Chrome's Local State os_crypt.encrypted_key via Windows DPAPI.
type dpapiRetriever struct{}

func (dpapiRetriever) Version() Version { return V10 }

func (dpapiRetriever) Retrieve(_ context.Context, loc Locator) ([]byte, error) {
	data, err := os.ReadFile(loc.LocalState)
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

	masterKey, err := winapi.DecryptDPAPI(keyBytes[len(dpapiPrefix):])
	if err != nil {
		return nil, fmt.Errorf("DPAPI decrypt: %w", err)
	}
	return masterKey, nil
}

// DecryptDPAPI decrypts a legacy Chromium value that has no version prefix, with the live user's DPAPI context.
func DecryptDPAPI(blob []byte) ([]byte, error) {
	return winapi.DecryptDPAPI(blob)
}
