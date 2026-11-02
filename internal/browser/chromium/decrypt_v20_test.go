package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// TestDecryptValue_V20 runs on every OS, since v20's format is the same everywhere, so all CI guards its routing.
func TestDecryptValue_V20(t *testing.T) {
	plaintext := []byte("v20_test_value")
	nonce := []byte("v20_nonce_12") // 12-byte AES-GCM nonce

	gcm, err := crypto.AESGCMEncrypt(testAESKey, nonce, plaintext)
	require.NoError(t, err)

	// v20 layout: "v20" (3B) + nonce (12B) + ciphertext+tag
	ciphertext := append([]byte("v20"), append(nonce, gcm...)...)

	got, err := decryptValue(masterkey.Set{masterkey.V20: testAESKey}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptValue_V20_ShortCiphertext(t *testing.T) {
	// Missing nonce (prefix only) must error, not panic.
	_, err := decryptValue(masterkey.Set{masterkey.V20: testAESKey}, []byte("v20"))
	require.Error(t, err)
}

// TestDecryptValue_V10_CrossHostGCM decrypts a v10 value sealed with a 32-byte key, as on Windows, on any host.
func TestDecryptValue_V10_CrossHostGCM(t *testing.T) {
	aesKey := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	plaintext := []byte("v10_cross_host")
	nonce := []byte("v10_nonce_12") // 12-byte AES-GCM nonce

	gcm, err := crypto.AESGCMEncrypt(aesKey, nonce, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v10"), append(nonce, gcm...)...)

	got, err := decryptValue(masterkey.Set{masterkey.V10: aesKey}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}
