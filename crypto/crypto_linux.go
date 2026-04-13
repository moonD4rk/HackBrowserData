//go:build linux

package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/sha1"
)

var chromiumCBCIV = bytes.Repeat([]byte{0x20}, aes.BlockSize)

// kEmptyKey is Chromium's decrypt-only fallback for data corrupted by a
// KWallet race in Chrome ~89 (crbug.com/40055416). Matches the kEmptyKey
// constant in os_crypt_linux.cc.
var kEmptyKey = PBKDF2Key([]byte(""), []byte("saltysalt"), 1, 16, sha1.New)

const minCBCDataSize = versionPrefixLen + aes.BlockSize // "v10" + one AES block = 19 bytes minimum

func DecryptChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minCBCDataSize {
		return nil, errShortCiphertext
	}
	payload := ciphertext[versionPrefixLen:]

	plaintext, err := AESCBCDecrypt(key, chromiumCBCIV, payload)
	if err == nil {
		return plaintext, nil
	}
	// Retry with kEmptyKey to recover crbug.com/40055416 data.
	if alt, altErr := AESCBCDecrypt(kEmptyKey, chromiumCBCIV, payload); altErr == nil {
		return alt, nil
	}
	return nil, err
}

func DecryptDPAPI(_ []byte) ([]byte, error) {
	return nil, errDPAPINotSupported
}
