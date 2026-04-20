package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

// TestDecryptValue_V20 is cross-platform because v20's ciphertext format
// (AES-GCM with 12-byte nonce) is platform-independent; only the key source
// (Chrome ABE on Windows) differs by OS. Running on Linux/macOS CI protects
// the routing in decryptValue + crypto.DecryptChromiumV20 from regressions.
func TestDecryptValue_V20(t *testing.T) {
	plaintext := []byte("v20_test_value")
	nonce := []byte("v20_nonce_12") // 12-byte AES-GCM nonce

	gcm, err := crypto.AESGCMEncrypt(testAESKey, nonce, plaintext)
	require.NoError(t, err)

	// v20 layout: "v20" (3B) + nonce (12B) + ciphertext+tag
	ciphertext := append([]byte("v20"), append(nonce, gcm...)...)

	got, err := decryptValue(keyretriever.MasterKeys{V20: testAESKey}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptValue_V20_ShortCiphertext(t *testing.T) {
	// Missing nonce (prefix only) must error, not panic.
	_, err := decryptValue(keyretriever.MasterKeys{V20: testAESKey}, []byte("v20"))
	require.Error(t, err)
}
