package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// v20's format is the same on every OS, so CI on Linux and macOS guards its routing too.
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

// A v10 value sealed with a 32-byte key, as on Windows, decrypts on any host: the cross-OS guarantee.
func TestDecryptValue_V10_CrossHostGCM(t *testing.T) {
	key32 := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	plaintext := []byte("v10_cross_host")
	nonce := []byte("v10_nonce_12") // 12-byte AES-GCM nonce

	gcm, err := crypto.AESGCMEncrypt(key32, nonce, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v10"), append(nonce, gcm...)...)

	got, err := decryptValue(masterkey.Set{masterkey.V10: key32}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}
