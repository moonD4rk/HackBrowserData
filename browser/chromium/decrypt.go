package chromium

import "github.com/moond4rk/hackbrowserdata/crypto"

// decryptValue decrypts a Chromium-encrypted value using the master key.
// It tries AES decryption first (v10 prefix), then falls back to DPAPI
// for legacy values without a version prefix.
func decryptValue(masterKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	value, err := crypto.DecryptWithChromium(masterKey, ciphertext)
	if err != nil {
		value, err = crypto.DecryptWithDPAPI(ciphertext)
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}
