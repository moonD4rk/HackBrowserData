package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/masterkey"
)

// TestDecryptValue_V20 is cross-platform because v20's ciphertext format
// (AES-GCM with 12-byte nonce) is platform-independent; only the key source
// (Chrome ABE on Windows) differs by OS. Running on Linux/macOS CI protects
// the routing in decryptValue + crypto.DecryptChromiumGCM from regressions.
func TestDecryptValue_V20(t *testing.T) {
	plaintext := []byte("v20_test_value")
	nonce := []byte("v20_nonce_12") // 12-byte AES-GCM nonce

	gcm, err := crypto.AESGCMEncrypt(testAESKey, nonce, plaintext)
	require.NoError(t, err)

	// v20 layout: "v20" (3B) + nonce (12B) + ciphertext+tag
	ciphertext := append([]byte("v20"), append(nonce, gcm...)...)

	got, err := decryptValue(masterkey.MasterKeys{V20: testAESKey}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptValue_V20_ShortCiphertext(t *testing.T) {
	// Missing nonce (prefix only) must error, not panic.
	_, err := decryptValue(masterkey.MasterKeys{V20: testAESKey}, []byte("v20"))
	require.Error(t, err)
}

// TestDecryptValue_V10_CrossHostGCM proves a v10 ciphertext sealed with a 32-byte
// AES-256 key (a Windows-origin dump) decrypts via decryptValue on any host — the
// core cross-OS guarantee. testAESKey is 16B, so this uses an explicit 32B key.
func TestDecryptValue_V10_CrossHostGCM(t *testing.T) {
	key32 := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	plaintext := []byte("v10_cross_host")
	nonce := []byte("v10_nonce_12") // 12-byte AES-GCM nonce

	gcm, err := crypto.AESGCMEncrypt(key32, nonce, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v10"), append(nonce, gcm...)...)

	got, err := decryptValue(masterkey.MasterKeys{V10: key32}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}
