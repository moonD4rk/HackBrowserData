package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"fmt"
)

func AESCBCEncrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcEncrypt(block, iv, plaintext)
}

func AESCBCDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcDecrypt(block, iv, ciphertext)
}

func DES3Encrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcEncrypt(block, iv, plaintext)
}

func DES3Decrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	return cbcDecrypt(block, iv, ciphertext)
}

const gcmNonceSize = 12

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

// blob is [12-byte nonce][ciphertext + 16-byte tag], as Yandex stores it.
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

func cbcEncrypt(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	if len(iv) != block.BlockSize() {
		return nil, errInvalidIVLength
	}

	padded := pkcs5Padding(plaintext, block.BlockSize())
	dst := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(dst, padded)
	return dst, nil
}

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

// paddingZero returns src unchanged if already long enough; otherwise a zero-padded new slice.
func paddingZero(src []byte, length int) []byte {
	if len(src) >= length {
		return src
	}
	dst := make([]byte, length)
	copy(dst, src)
	return dst
}

// pkcs5Padding always returns a new slice and never modifies src.
func pkcs5Padding(src []byte, blockSize int) []byte {
	n := blockSize - (len(src) % blockSize)
	dst := make([]byte, len(src)+n)
	copy(dst, src)
	for i := len(src); i < len(dst); i++ {
		dst[i] = byte(n)
	}
	return dst
}

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
