package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"errors"
	"fmt"
)

var ErrCiphertextLengthIsInvalid = errors.New("ciphertext length is invalid")

func AES128CBCDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// Check ciphertext length
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("AES128CBCDecrypt: ciphertext too short")
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("AES128CBCDecrypt: ciphertext is not a multiple of the block size")
	}

	decryptedData := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decryptedData, ciphertext)

	// unpad the decrypted data and handle potential padding errors
	decryptedData, err = pkcs5UnPadding(decryptedData)
	if err != nil {
		return nil, fmt.Errorf("AES128CBCDecrypt: %w", err)
	}

	return decryptedData, nil
}

func AES128CBCEncrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(iv) != aes.BlockSize {
		return nil, errors.New("AES128CBCEncrypt: iv length is invalid, must equal block size")
	}

	plaintext = pkcs5Padding(plaintext, block.BlockSize())
	encryptedData := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encryptedData, plaintext)

	return encryptedData, nil
}

func DES3Decrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < des.BlockSize {
		return nil, errors.New("DES3Decrypt: ciphertext too short")
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, errors.New("DES3Decrypt: ciphertext is not a multiple of the block size")
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	sq := make([]byte, len(ciphertext))
	blockMode.CryptBlocks(sq, ciphertext)

	return pkcs5UnPadding(sq)
}

func DES3Encrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}

	plaintext = pkcs5Padding(plaintext, block.BlockSize())
	dst := make([]byte, len(plaintext))
	blockMode := cipher.NewCBCEncrypter(block, iv)
	blockMode.CryptBlocks(dst, plaintext)

	return dst, nil
}

// AESGCMDecrypt chromium > 80 https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_win.cc
func AESGCMDecrypt(key, nounce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	origData, err := blockMode.Open(nil, nounce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return origData, nil
}

// AESGCMEncrypt encrypts plaintext using AES encryption in GCM mode.
func AESGCMEncrypt(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// The first parameter is the prefix for the output, we can leave it nil.
	// The Seal method encrypts and authenticates the data, appending the result to the dst.
	encryptedData := blockMode.Seal(nil, nonce, plaintext, nil)
	return encryptedData, nil
}

func paddingZero(src []byte, length int) []byte {
	padding := length - len(src)
	if padding <= 0 {
		return src
	}
	return append(src, make([]byte, padding)...)
}

func pkcs5UnPadding(src []byte) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, errors.New("pkcs5UnPadding: src should not be empty")
	}
	padding := int(src[length-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, errors.New("pkcs5UnPadding: invalid padding size")
	}
	return src[:length-padding], nil
}

func pkcs5Padding(src []byte, blocksize int) []byte {
	padding := blocksize - (len(src) % blocksize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padText...)
}
