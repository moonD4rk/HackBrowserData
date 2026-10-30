package crypto

import (
	"bytes"
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

// --- Bug-fix verification tests ---
// These tests verify the fixes for known issues in the crypto primitives.
// Tests marked with t.Skip document bugs that exist before the fix.

func TestPkcs5Padding_NoMutation(t *testing.T) {
	// pkcs5Padding should not mutate the original slice's backing array.
	src := make([]byte, 3, 32) // len=3, cap=32 — append won't allocate
	copy(src, "abc")
	backup := make([]byte, cap(src))
	copy(backup, src[:cap(src)])

	padded := pkcs5Padding(src, 16)
	assert.Len(t, padded, 16)
	assert.Equal(t, []byte("abc"), src) // original length unchanged

	// The bytes beyond len(src) in the original backing array must not be touched.
	current := make([]byte, cap(src))
	copy(current, src[:cap(src)])
	assert.Equal(t, backup, current, "pkcs5Padding mutated the original slice backing array")
}

func TestPaddingZero_NoMutation(t *testing.T) {
	src := make([]byte, 3, 32)
	copy(src, "abc")
	backup := make([]byte, cap(src))
	copy(backup, src[:cap(src)])

	padded := paddingZero(src, 20)
	assert.Len(t, padded, 20)

	current := make([]byte, cap(src))
	copy(current, src[:cap(src)])
	assert.Equal(t, backup, current, "paddingZero mutated the original slice backing array")
}

func TestAESCBCDecrypt_WrongIVLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	wrongIV := []byte("short")
	ct := make([]byte, 16)

	_, err := AESCBCDecrypt(key, wrongIV, ct)
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

func TestDES3Decrypt_WrongIVLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 24)
	wrongIV := []byte("ab") // DES needs 8-byte IV
	ct := make([]byte, 8)

	_, err := DES3Decrypt(key, wrongIV, ct)
	require.Error(t, err, "wrong IV length should return error, not panic")
	assert.ErrorIs(t, err, errInvalidIVLength)
}

func TestDES3Encrypt_WrongIVLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 24)
	wrongIV := []byte("ab")

	_, err := DES3Encrypt(key, wrongIV, plainText)
	require.Error(t, err, "wrong IV length should return error, not panic")
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
	assert.ErrorIs(t, err, errInvalidNonceLen)
}

func TestAESGCMDecrypt_WrongNonceLength(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 16)
	wrongNonce := []byte("short")
	ct := make([]byte, 32)

	_, err := AESGCMDecrypt(key, wrongNonce, ct)
	require.Error(t, err, "wrong nonce length should return error, not panic")
	assert.ErrorIs(t, err, errInvalidNonceLen)
}

func TestDES3Decrypt_EmptyCiphertext(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 24)
	iv := bytes.Repeat([]byte("i"), 8)

	_, err := DES3Decrypt(key, iv, nil)
	require.Error(t, err)

	_, err = DES3Decrypt(key, iv, []byte{})
	require.Error(t, err)
}
