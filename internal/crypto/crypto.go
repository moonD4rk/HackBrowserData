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

// AESGCMDecryptBlob takes [12-byte nonce][ciphertext + 16-byte tag], as Yandex stores it.
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
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)
	return encrypted, nil
}

func cbcDecrypt(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	blockSize := block.BlockSize()
	if len(iv) != blockSize {
		return nil, errInvalidIVLength
	}
	if len(ciphertext) < blockSize {
		return nil, errShortCiphertext
	}
	if len(ciphertext)%blockSize != 0 {
		return nil, errInvalidBlockSize
	}

	decrypted := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, ciphertext)

	decrypted, err := pkcs5UnPadding(decrypted, blockSize)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return decrypted, nil
}

// paddingZero returns data unchanged if already long enough; otherwise a zero-padded new slice.
func paddingZero(data []byte, length int) []byte {
	if len(data) >= length {
		return data
	}
	padded := make([]byte, length)
	copy(padded, data)
	return padded
}

// pkcs5Padding always returns a new slice and never modifies data.
func pkcs5Padding(data []byte, blockSize int) []byte {
	paddingLength := blockSize - (len(data) % blockSize)
	padded := make([]byte, len(data)+paddingLength)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(paddingLength)
	}
	return padded
}

func pkcs5UnPadding(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errInvalidPadding
	}
	padding := int(data[length-1])
	if padding < 1 || padding > blockSize || padding > length {
		return nil, errInvalidPadding
	}
	for _, paddingByte := range data[length-padding:] {
		if int(paddingByte) != padding {
			return nil, errInvalidPadding
		}
	}
	return data[:length-padding], nil
}
