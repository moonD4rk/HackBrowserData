package datautil

import "github.com/moond4rk/hackbrowserdata/crypto"

// DecryptChromiumValue decrypts a Chromium-encrypted value using the master key.
//
// It tries DPAPI first (Windows), then falls back to Chromium AES-GCM/CBC.
// If masterKey is empty, only DPAPI is attempted (Yandex browser behavior on Windows).
//
// Returns nil for empty input without error.
func DecryptChromiumValue(masterKey, encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, nil
	}
	if len(masterKey) == 0 {
		return crypto.DecryptWithDPAPI(encrypted)
	}
	value, err := crypto.DecryptWithDPAPI(encrypted)
	if err != nil {
		value, err = crypto.DecryptWithChromium(masterKey, encrypted)
	}
	return value, err
}
