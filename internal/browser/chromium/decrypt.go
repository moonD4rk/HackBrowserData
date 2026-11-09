package chromium

import (
	"bytes"
	"crypto/aes"
	"crypto/sha1"
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// decryptValue fails only the values sealed under a version whose key is missing, since a profile can mix versions.
func decryptValue(masterKeys masterkey.Set, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	version := detectVersion(ciphertext)
	switch version {
	case masterkey.V10:
		// Key length tells which platform sealed v10: 32 bytes is GCM, 16 is CBC, so a key from another OS still works.
		if len(masterKeys[masterkey.V10]) == 32 {
			return decryptGCM(masterKeys[masterkey.V10], ciphertext)
		}
		return decryptCBC(masterKeys[masterkey.V10], ciphertext)
	case masterkey.V11:
		return decryptCBC(masterKeys[masterkey.V11], ciphertext)
	case masterkey.V20:
		return decryptGCM(masterKeys[masterkey.V20], ciphertext)
	case versionV12:
		return nil, fmt.Errorf("unsupported cipher version v12 (Chromium SecretPortal / Flatpak; not yet implemented)")
	case unversioned:
		return masterkey.DecryptDPAPI(ciphertext)
	default:
		return nil, fmt.Errorf("unsupported cipher version: %s", version)
	}
}

const (
	// versionV12 is recognized only to report a known gap: v12 is not implemented yet.
	versionV12 masterkey.Version = "v12"
	// unversioned is a value without a version prefix, which Chromium on Windows sealed with DPAPI.
	unversioned masterkey.Version = ""

	versionPrefixLength = 3
)

func detectVersion(ciphertext []byte) masterkey.Version {
	if len(ciphertext) < versionPrefixLength {
		return unversioned
	}
	switch version := masterkey.Version(ciphertext[:versionPrefixLength]); version {
	case masterkey.V10, masterkey.V11, masterkey.V20, versionV12:
		return version
	default:
		return unversioned
	}
}

var errShortCiphertext = errors.New("ciphertext too short")

const gcmNonceSize = 12

var cbcIV = bytes.Repeat([]byte{0x20}, aes.BlockSize)

// emptyPasswordKey is Chromium's decrypt-only fallback for data a KWallet race corrupted (crbug.com/40055416).
var emptyPasswordKey = crypto.PBKDF2Key([]byte(""), []byte("saltysalt"), 1, 16, sha1.New)

func decryptGCM(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < versionPrefixLength+gcmNonceSize {
		return nil, errShortCiphertext
	}
	nonce := ciphertext[versionPrefixLength : versionPrefixLength+gcmNonceSize]
	payload := ciphertext[versionPrefixLength+gcmNonceSize:]
	return crypto.AESGCMDecrypt(key, nonce, payload)
}

func decryptCBC(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < versionPrefixLength+aes.BlockSize {
		return nil, errShortCiphertext
	}
	payload := ciphertext[versionPrefixLength:]
	plaintext, err := crypto.AESCBCDecrypt(key, cbcIV, payload)
	if err == nil {
		return plaintext, nil
	}
	if fallback, fallbackErr := crypto.AESCBCDecrypt(emptyPasswordKey, cbcIV, payload); fallbackErr == nil {
		return fallback, nil
	}
	return nil, err
}

// masterKeysFunc returns the dir's master keys, which the first call may retrieve; an empty value never calls it.
type masterKeysFunc func() masterkey.Set

func (keys masterKeysFunc) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	return decryptValue(keys(), ciphertext)
}
