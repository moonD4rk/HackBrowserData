package chromium

import (
	"bytes"
	"crypto/aes"
	"crypto/sha1"
	"errors"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
)

type cipherVersion string

const (
	cipherV10 cipherVersion = "v10"

	cipherV11 cipherVersion = "v11"

	cipherV20 cipherVersion = "v20"

	// cipherV12 is recognized only to report a known gap: v12 is not implemented yet.
	cipherV12 cipherVersion = "v12"

	cipherDPAPI cipherVersion = "dpapi"

	versionPrefixLength = 3
)

func detectVersion(ciphertext []byte) cipherVersion {
	if len(ciphertext) < versionPrefixLength {
		return cipherDPAPI
	}
	prefix := string(ciphertext[:versionPrefixLength])
	switch prefix {
	case "v10":
		return cipherV10
	case "v11":
		return cipherV11
	case "v12":
		return cipherV12
	case "v20":
		return cipherV20
	default:
		return cipherDPAPI
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
