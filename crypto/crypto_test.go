package crypto

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const baseKey = "moond4rk"

var (
	aesKey           = bytes.Repeat([]byte(baseKey), 2) // 16 bytes
	aesIV            = []byte("01234567abcdef01")       // 16 bytes
	plainText        = []byte("Hello, World!")
	aes128Ciphertext = "19381468ecf824c0bfc7a89eed9777d2"

	des3Key        = sha1.New().Sum(aesKey)[:24]
	des3IV         = aesIV[:8]
	des3Ciphertext = "a4492f31bc404fae18d53a46ca79282e"

	aesGCMNonce      = aesKey[:12]
	aesGCMCiphertext = "6c49dac89992639713edab3a114c450968a08b53556872cea3919e2e9a"
)

func TestAES128CBCEncrypt(t *testing.T) {
	encrypted, err := AES128CBCEncrypt(aesKey, aesIV, plainText)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, len(encrypted) > 0)
	assert.Equal(t, aes128Ciphertext, fmt.Sprintf("%x", encrypted))
}

func TestAES128CBCDecrypt(t *testing.T) {
	ciphertext, _ := hex.DecodeString(aes128Ciphertext)
	decrypted, err := AES128CBCDecrypt(aesKey, aesIV, ciphertext)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, len(decrypted) > 0)
	assert.Equal(t, plainText, decrypted)
}

func TestDES3Encrypt(t *testing.T) {
	encrypted, err := DES3Encrypt(des3Key, des3IV, plainText)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, len(encrypted) > 0)
	assert.Equal(t, des3Ciphertext, fmt.Sprintf("%x", encrypted))
}

func TestDES3Decrypt(t *testing.T) {
	ciphertext, _ := hex.DecodeString(des3Ciphertext)
	decrypted, err := DES3Decrypt(des3Key, des3IV, ciphertext)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, len(decrypted) > 0)
	assert.Equal(t, plainText, decrypted)
}

func TestAESGCMEncrypt(t *testing.T) {
	encrypted, err := AESGCMEncrypt(aesKey, aesGCMNonce, plainText)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, len(encrypted) > 0)
	assert.Equal(t, aesGCMCiphertext, fmt.Sprintf("%x", encrypted))
}

func TestAESGCMDecrypt(t *testing.T) {
	ciphertext, _ := hex.DecodeString(aesGCMCiphertext)
	decrypted, err := AESGCMDecrypt(aesKey, aesGCMNonce, ciphertext)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, len(decrypted) > 0)
	assert.Equal(t, plainText, decrypted)
}
