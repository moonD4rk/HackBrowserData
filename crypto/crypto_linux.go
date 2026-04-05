//go:build linux

package crypto

func DecryptWithChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 3 {
		return nil, ErrCiphertextLengthIsInvalid
	}
	iv := []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	return AESCBCDecrypt(key, iv, ciphertext[3:])
}

func DecryptWithDPAPI(_ []byte) ([]byte, error) {
	return nil, nil
}
