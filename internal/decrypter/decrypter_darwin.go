package decrypter

var (
	errSecurityKeyIsEmpty = errors.New("input [security find-generic-password -wa 'Chrome'] in terminal")
)

func Chromium(key, encryptPass []byte) ([]byte, error) {
	if len(encryptPass) <= 3 {
		return nil, errPasswordIsEmpty
	}
	if len(key) == 0 {
		return nil, errSecurityKeyIsEmpty
	}

	iv := []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	return aes128CBCDecrypt(key, iv, encryptPass[3:])
}

func DPAPI(data []byte) ([]byte, error) {
	return nil, nil
}
