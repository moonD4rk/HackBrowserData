package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// TestDecryptValue_MixedTier guards #578: each value in a mixed-version profile routes to its own version's key.
func TestDecryptValue_MixedTier(t *testing.T) {
	v10Key := bytes.Repeat([]byte{0x10}, 16) // V10 slot key (wrong for v20 payload)
	v11Key := bytes.Repeat([]byte{0x11}, 16) // V11 slot key (wrong for v20 payload)
	v20Key := bytes.Repeat([]byte{0x20}, 16) // V20 slot key (correct for v20 payload)

	plaintext := []byte("cookie-value-encrypted-with-k20")
	nonce := []byte("v20_nonce_12") // 12-byte AES-GCM nonce

	gcmEnc, err := crypto.AESGCMEncrypt(v20Key, nonce, plaintext)
	require.NoError(t, err)
	v20Ciphertext := append([]byte("v20"), append(nonce, gcmEnc...)...)

	t.Run("all tiers populated: v20 picks V20, decrypts", func(t *testing.T) {
		got, err := decryptValue(masterkey.Set{masterkey.V10: v10Key, masterkey.V11: v11Key, masterkey.V20: v20Key}, v20Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, got)
	})

	t.Run("V20 holds wrong key: v20 still picks V20 slot (not V10/V11), errors", func(t *testing.T) {
		// Wrong keys in the V20 slot must fail even when another slot holds the right one: routing is by prefix.
		_, err := decryptValue(masterkey.Set{masterkey.V10: v20Key, masterkey.V11: v20Key, masterkey.V20: v10Key}, v20Ciphertext)
		require.Error(t, err)
	})

	t.Run("only V20 populated: v20 still decrypts", func(t *testing.T) {
		// Missing V10 and V11 keys must not block v20 values.
		got, err := decryptValue(masterkey.Set{masterkey.V20: v20Key}, v20Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, got)
	})

	t.Run("V20 slot unpopulated: v20 errors (no key to use)", func(t *testing.T) {
		_, err := decryptValue(masterkey.Set{masterkey.V10: v10Key, masterkey.V11: v11Key}, v20Ciphertext)
		require.Error(t, err)
	})
}
