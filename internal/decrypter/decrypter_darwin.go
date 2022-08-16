//go:build darwin

package decrypter

func Chromium(key, encryptPass []byte) ([]byte, error) {
	if len(encryptPass) <= 3 {
		return nil, errPasswordIsEmpty
	}

	iv := []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	return aes128CBCDecrypt(key, iv, encryptPass[3:])
}

func DPAPI(data []byte) ([]byte, error) {
	return nil, nil
}
