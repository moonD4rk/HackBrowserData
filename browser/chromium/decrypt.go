package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

// decryptValue decrypts a Chromium-encrypted value using the master key.
// It detects the cipher version from the ciphertext prefix and routes
// to the appropriate decryption function.
func decryptValue(masterKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	version := crypto.DetectVersion(ciphertext)
	switch version {
	case crypto.CipherV10, crypto.CipherV11:
		// v11 is Linux-only and shares v10's AES-CBC path; only the key source differs.
		return crypto.DecryptChromium(masterKey, ciphertext)
	case crypto.CipherV20:
		return crypto.DecryptChromium(masterKey, ciphertext)
	case crypto.CipherDPAPI:
		return crypto.DecryptDPAPI(ciphertext)
	default:
		return nil, fmt.Errorf("unsupported cipher version: %s", version)
	}
}
