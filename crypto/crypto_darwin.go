//go:build darwin

package crypto

import (
	"bytes"
	"crypto/aes"
)

var chromiumCBCIV = bytes.Repeat([]byte{0x20}, aes.BlockSize)

const minCBCDataSize = versionPrefixLen + aes.BlockSize // "v10" + one AES block = 19 bytes minimum

func DecryptChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minCBCDataSize {
		return nil, errShortCiphertext
	}
	return AESCBCDecrypt(key, chromiumCBCIV, ciphertext[versionPrefixLen:])
}

func DecryptDPAPI(_ []byte) ([]byte, error) {
	return nil, errDPAPINotSupported
}
