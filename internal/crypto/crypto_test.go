package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAESCBCEncrypt(t *testing.T) {
	encrypted, err := AESCBCEncrypt(aesKey, aesIV, plainText)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.Equal(t, aes128Ciphertext, fmt.Sprintf("%x", encrypted))
}

func TestAESCBCDecrypt(t *testing.T) {
	ciphertext, _ := hex.DecodeString(aes128Ciphertext)
	decrypted, err := AESCBCDecrypt(aesKey, aesIV, ciphertext)
	require.NoError(t, err)
	assert.NotEmpty(t, decrypted)
	assert.Equal(t, plainText, decrypted)
}

func TestDES3Encrypt(t *testing.T) {
	encrypted, err := DES3Encrypt(des3Key, des3IV, plainText)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.Equal(t, des3Ciphertext, fmt.Sprintf("%x", encrypted))
}

func TestDES3Decrypt(t *testing.T) {
	ciphertext, _ := hex.DecodeString(des3Ciphertext)
	decrypted, err := DES3Decrypt(des3Key, des3IV, ciphertext)
	require.NoError(t, err)
	assert.NotEmpty(t, decrypted)
	assert.Equal(t, plainText, decrypted)
}

func TestAESGCMEncrypt(t *testing.T) {
	encrypted, err := AESGCMEncrypt(aesKey, aesGCMNonce, plainText)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.Equal(t, aesGCMCiphertext, fmt.Sprintf("%x", encrypted))
}

func TestAESGCMDecrypt(t *testing.T) {
	ciphertext, _ := hex.DecodeString(aesGCMCiphertext)
	decrypted, err := AESGCMDecrypt(aesKey, aesGCMNonce, ciphertext)
	require.NoError(t, err)
	assert.NotEmpty(t, decrypted)
	assert.Equal(t, plainText, decrypted)
}

func TestAESGCMDecryptBlob_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	nonce := bytes.Repeat([]byte{0x66}, gcmNonceSize)
	aad := []byte("row-aad")
	blob := append(append([]byte{}, nonce...), encryptAESGCM(t, key, nonce, []byte("row-plaintext"), aad)...)

	got, err := AESGCMDecryptBlob(key, blob, aad)
	require.NoError(t, err)
	assert.Equal(t, []byte("row-plaintext"), got)
}

func TestAESGCMDecryptBlob_BadAAD(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	nonce := bytes.Repeat([]byte{0x66}, gcmNonceSize)
	blob := append(append([]byte{}, nonce...), encryptAESGCM(t, key, nonce, []byte("x"), []byte("aad-A"))...)

	_, err := AESGCMDecryptBlob(key, blob, []byte("aad-B"))
	require.Error(t, err, "a mismatched AAD fails authentication")
}

func TestAESGCMDecryptBlob_TooShort(t *testing.T) {
	_, err := AESGCMDecryptBlob(bytes.Repeat([]byte{0x55}, 32), []byte{0x01, 0x02}, nil)
	require.ErrorIs(t, err, errShortCiphertext)
}

func encryptAESGCM(t *testing.T, key, nonce, plaintext, aad []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	return aead.Seal(nil, nonce, plaintext, aad)
}

func TestPKCS5Pad_NoMutation(t *testing.T) {
	data := make([]byte, 3, 32) // len=3, cap=32 — append won't allocate
	copy(data, "abc")
	backup := make([]byte, cap(data))
	copy(backup, data[:cap(data)])

	padded := pkcs5Pad(data, 16)
	assert.Len(t, padded, 16)
	assert.Equal(t, []byte("abc"), data)

	// The bytes beyond len(data) in the original backing array must not be touched.
	current := make([]byte, cap(data))
	copy(current, data[:cap(data)])
	assert.Equal(t, backup, current, "pkcs5Pad mutated the original slice backing array")
}

func TestZeroPad_NoMutation(t *testing.T) {
	data := make([]byte, 3, 32)
	copy(data, "abc")
	backup := make([]byte, cap(data))
	copy(backup, data[:cap(data)])

	padded := zeroPad(data, 20)
	assert.Equal(t, append([]byte("abc"), make([]byte, 17)...), padded)

	current := make([]byte, cap(data))
	copy(current, data[:cap(data)])
	assert.Equal(t, backup, current, "zeroPad mutated the original slice backing array")
}

func TestAESCBCDecrypt_WrongIVLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	wrongIV := []byte("short")
	ciphertext := make([]byte, 16)

	_, err := AESCBCDecrypt(key, wrongIV, ciphertext)
	require.Error(t, err, "wrong IV length should return error, not panic")
	assert.ErrorIs(t, err, errInvalidIVLength)
}

func TestAESCBCEncrypt_WrongIVLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	wrongIV := []byte("short")

	_, err := AESCBCEncrypt(key, wrongIV, plainText)
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidIVLength)
}

func TestAESCBCDecrypt_EmptyCiphertext(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	iv := bytes.Repeat([]byte("i"), 16)

	_, err := AESCBCDecrypt(key, iv, nil)
	require.Error(t, err)

	_, err = AESCBCDecrypt(key, iv, []byte{})
	require.Error(t, err)
}

func TestAESGCMEncrypt_WrongNonceLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	wrongNonce := []byte("short")

	_, err := AESGCMEncrypt(key, wrongNonce, plainText)
	require.Error(t, err, "wrong nonce length should return error, not panic")
	assert.ErrorIs(t, err, errInvalidNonceLength)
}

func TestAESGCMDecrypt_WrongNonceLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	wrongNonce := []byte("short")
	ciphertext := make([]byte, 32)

	_, err := AESGCMDecrypt(key, wrongNonce, ciphertext)
	require.Error(t, err, "wrong nonce length should return error, not panic")
	assert.ErrorIs(t, err, errInvalidNonceLength)
}
