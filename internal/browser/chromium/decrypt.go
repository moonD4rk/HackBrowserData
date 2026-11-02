package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// decryptValue fails only the values sealed under a version whose key is missing, since a profile can mix versions.
func decryptValue(masterKeys masterkey.Set, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	version := detectVersion(ciphertext)
	switch version {
	case cipherV10:
		// Key length tells which platform sealed v10: 32 bytes is GCM, 16 is CBC, so a key from another OS still works.
		if len(masterKeys[masterkey.V10]) == 32 {
			return decryptGCM(masterKeys[masterkey.V10], ciphertext)
		}
		return decryptCBC(masterKeys[masterkey.V10], ciphertext)
	case cipherV11:
		return decryptCBC(masterKeys[masterkey.V11], ciphertext)
	case cipherV20:
		return decryptGCM(masterKeys[masterkey.V20], ciphertext)
	case cipherV12:
		return nil, fmt.Errorf("unsupported cipher version v12 (Chromium SecretPortal / Flatpak; not yet implemented)")
	case cipherDPAPI:
		return masterkey.DecryptDPAPI(ciphertext)
	default:
		return nil, fmt.Errorf("unsupported cipher version: %s", version)
	}
}
