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
	case crypto.CipherV10:
		return crypto.DecryptWithChromium(masterKey, ciphertext)
	case crypto.CipherV20:
		// TODO: implement App-Bound Encryption (Chrome 127+)
		return nil, fmt.Errorf("v20 App-Bound Encryption not yet supported")
	case crypto.CipherDPAPI:
		return crypto.DecryptWithDPAPI(ciphertext)
	default:
		return nil, fmt.Errorf("unsupported cipher version: %s", version)
	}
}
