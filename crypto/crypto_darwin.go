//go:build darwin

package crypto

import "errors"

var ErrDarwinNotSupportDPAPI = errors.New("darwin not support dpapi")

func DecryptWithChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) <= 3 {
		return nil, ErrCiphertextLengthIsInvalid
	}
	iv := []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	return AESCBCDecrypt(key, iv, ciphertext[3:])
}

func DecryptWithDPAPI(_ []byte) ([]byte, error) {
	return nil, ErrDarwinNotSupportDPAPI
}
