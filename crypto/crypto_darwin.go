//go:build darwin

package crypto

import "errors"

var ErrDarwinNotSupportDPAPI = errors.New("darwin not support dpapi")

func DecryptWithChromium(key, password []byte) ([]byte, error) {
	if len(password) <= 3 {
		return nil, ErrCiphertextLengthIsInvalid
	}
	iv := []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	return AES128CBCDecrypt(key, iv, password[3:])
}

func DecryptWithDPAPI(_ []byte) ([]byte, error) {
	return nil, ErrDarwinNotSupportDPAPI
}
