//go:build linux

package crypto

var iv = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}

func DecryptPass(key, encryptPass []byte) ([]byte, error) {
	if len(encryptPass) < 3 {
		return nil, errPasswordIsEmpty
	}
	return aes128CBCDecrypt(key, iv, encryptPass[3:])
}

func DPAPI(_ []byte) ([]byte, error) {
	return nil, nil
}
