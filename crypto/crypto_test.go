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

// --- Cross-OS Chromium v10/v11 decryption ---
// DecryptChromiumGCM / DecryptChromiumCBC are platform-neutral, so these run on every
// GOOS and prove a key dumped on one platform decrypts that platform's data anywhere.

// key32 is the 32-byte AES-256-GCM tier (Windows v10 / v20); aesKey is the 16-byte
// AES-128-CBC tier (macOS/Linux v10/v11).
var key32 = bytes.Repeat([]byte(baseKey), 4)

func TestDecryptChromiumGCM_CrossPlatform(t *testing.T) {
	plaintext := []byte("windows_v10_value")
	gcm, err := AESGCMEncrypt(key32, aesGCMNonce, plaintext)
	require.NoError(t, err)

	ciphertext := append([]byte("v10"), append(aesGCMNonce, gcm...)...)
	got, err := DecryptChromiumGCM(key32, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptChromiumCBC_CrossPlatform(t *testing.T) {
	plaintext := []byte("posix_v10_value")
	enc, err := AESCBCEncrypt(aesKey, chromiumCBCIV, plaintext)
	require.NoError(t, err)

	ciphertext := append([]byte("v10"), enc...)
	got, err := DecryptChromiumCBC(aesKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

// TestKEmptyKey_MatchesChromium pins the runtime-derived kEmptyKey to Chromium's
// reference bytes in os_crypt_linux.cc; now cross-platform since kEmptyKey is
// defined for every GOOS.
func TestKEmptyKey_MatchesChromium(t *testing.T) {
	want := []byte{
		0xd0, 0xd0, 0xec, 0x9c, 0x7d, 0x77, 0xd4, 0x3a,
		0xc5, 0x41, 0x87, 0xfa, 0x48, 0x18, 0xd1, 0x7f,
	}
	assert.Equal(t, want, kEmptyKey)
	assert.Len(t, kEmptyKey, 16)
}

func TestDecryptChromiumCBC_EmptyKeyFallback(t *testing.T) {
	plaintext := []byte("legacy_kwallet_value")
	encrypted, err := AESCBCEncrypt(kEmptyKey, chromiumCBCIV, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v11"), encrypted...)

	wrongKey := bytes.Repeat([]byte{0xAA}, 16)
	got, err := DecryptChromiumCBC(wrongKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptChromium_ShortCiphertext(t *testing.T) {
	// GCM minimum is prefix(3)+nonce(12) = 15 bytes.
	_, err := DecryptChromiumGCM(key32, []byte("v10nonce11"))
	require.ErrorIs(t, err, errShortCiphertext)

	// CBC minimum is prefix(3)+block(16) = 19 bytes.
	_, err = DecryptChromiumCBC(aesKey, []byte("v11short"))
	require.ErrorIs(t, err, errShortCiphertext)
}
