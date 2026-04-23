package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"fmt"
)

// AESCBCEncrypt encrypts data using AES-CBC mode with PKCS5 padding.
// Supports all AES key sizes: 16 bytes (AES-128), 24 bytes (AES-192), or 32 bytes (AES-256).
func AESCBCEncrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcEncrypt(block, iv, plaintext)
}

// AESCBCDecrypt decrypts data using AES-CBC mode with PKCS5 unpadding.
// Supports all AES key sizes: 16 bytes (AES-128), 24 bytes (AES-192), or 32 bytes (AES-256).
func AESCBCDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcDecrypt(block, iv, ciphertext)
}

// DES3Encrypt encrypts data using 3DES-CBC mode with PKCS5 padding.
func DES3Encrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcEncrypt(block, iv, plaintext)
}

// DES3Decrypt decrypts data using 3DES-CBC mode with PKCS5 unpadding.
func DES3Decrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcDecrypt(block, iv, ciphertext)
}

// gcmNonceSize is the AES-GCM standard nonce size used by Chromium's v10/v20
// cipher formats. Cross-platform because the v20 ciphertext layout is the
// same regardless of host OS (only Windows currently produces v20).
const gcmNonceSize = 12

// DecryptChromiumV20 decrypts a Chromium v20 (App-Bound Encryption) ciphertext.
// Format: "v20" prefix (3B) + nonce (12B) + AES-GCM(payload + 16B tag).
//
// Cross-platform: v20 is only produced by Chrome on Windows today, but the
// decryption math is platform-neutral. Keeping it here rather than in
// crypto_windows.go ensures the routing in browser/chromium/decrypt.go stays
// testable on Linux/macOS CI.
func DecryptChromiumV20(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < versionPrefixLen+gcmNonceSize {
		return nil, errShortCiphertext
	}
	nonce := ciphertext[versionPrefixLen : versionPrefixLen+gcmNonceSize]
	payload := ciphertext[versionPrefixLen+gcmNonceSize:]
	return AESGCMDecrypt(key, nonce, payload)
}

// AESGCMEncrypt encrypts data using AES-GCM mode.
func AESGCMEncrypt(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, errInvalidNonceLen
	}
	return aead.Seal(nil, nonce, plaintext, nil), nil
}

// AESGCMDecrypt decrypts data using AES-GCM mode.
func AESGCMDecrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, errInvalidNonceLen
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}

// AESGCMDecryptBlob decrypts a blob shaped as [12B nonce][ciphertext+16B GCM tag] with caller-supplied AAD.
// Used by protocols that wrap AES-GCM output with a fixed-length nonce prefix (Yandex passwords/cards).
func AESGCMDecryptBlob(key, blob, aad []byte) ([]byte, error) {
	if len(blob) < gcmNonceSize {
		return nil, errShortCiphertext
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, blob[:gcmNonceSize], blob[gcmNonceSize:], aad)
}

// cbcEncrypt adds PKCS5 padding and encrypts plaintext in CBC mode.
func cbcEncrypt(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	if len(iv) != block.BlockSize() {
		return nil, errInvalidIVLength
	}

	padded := pkcs5Padding(plaintext, block.BlockSize())
	dst := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(dst, padded)
	return dst, nil
}

// cbcDecrypt decrypts ciphertext in CBC mode and removes PKCS5 padding.
func cbcDecrypt(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	bs := block.BlockSize()
	if len(iv) != bs {
		return nil, errInvalidIVLength
	}
	if len(ciphertext) < bs {
		return nil, errShortCiphertext
	}
	if len(ciphertext)%bs != 0 {
		return nil, errInvalidBlockSize
	}

	dst := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(dst, ciphertext)

	dst, err := pkcs5UnPadding(dst, bs)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return dst, nil
}

// paddingZero pads src with zero bytes to the given length.
// Returns src unchanged if already long enough; otherwise returns a new slice.
func paddingZero(src []byte, length int) []byte {
	if len(src) >= length {
		return src
	}
	dst := make([]byte, length)
	copy(dst, src)
	return dst
}

// pkcs5Padding adds PKCS5/PKCS7 padding to src.
// Always returns a new slice; never modifies src.
func pkcs5Padding(src []byte, blockSize int) []byte {
	n := blockSize - (len(src) % blockSize)
	dst := make([]byte, len(src)+n)
	copy(dst, src)
	for i := len(src); i < len(dst); i++ {
		dst[i] = byte(n)
	}
	return dst
}

// pkcs5UnPadding removes PKCS5/PKCS7 padding from src.
func pkcs5UnPadding(src []byte, blockSize int) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, errInvalidPadding
	}
	padding := int(src[length-1])
	if padding < 1 || padding > blockSize || padding > length {
		return nil, errInvalidPadding
	}
	for _, b := range src[length-padding:] {
		if int(b) != padding {
			return nil, errInvalidPadding
		}
	}
	return src[:length-padding], nil
}
