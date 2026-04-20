package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

// TestDecryptValue_MixedTier is the regression test for mixed-cipher profiles (issue #578 on
// Windows; the analogous Linux v10/v11 gap). A single MasterKeys struct must carry distinct keys
// for each tier, and decryptValue must dispatch each ciphertext to the matching tier's key.
// Before the refactor the master-key retriever returned only one tier, so a profile mixing
// cipher prefixes silently lost whichever tier wasn't retrieved.
//
// Uses v20 (cross-platform AES-GCM) to cover the prefix→slot routing property without depending
// on platform-specific v10/v11 cipher primitives (AES-CBC on darwin/linux, AES-GCM on Windows).
// The per-platform v10/v11 formats are covered by decrypt_test.go and decrypt_windows_test.go.
func TestDecryptValue_MixedTier(t *testing.T) {
	k10 := bytes.Repeat([]byte{0x10}, 16) // V10 slot key (wrong for v20 payload)
	k11 := bytes.Repeat([]byte{0x11}, 16) // V11 slot key (wrong for v20 payload)
	k20 := bytes.Repeat([]byte{0x20}, 16) // V20 slot key (correct for v20 payload)

	plaintext := []byte("cookie-value-encrypted-with-k20")
	nonce := []byte("v20_nonce_12") // 12-byte AES-GCM nonce

	gcmEnc, err := crypto.AESGCMEncrypt(k20, nonce, plaintext)
	require.NoError(t, err)
	v20Ciphertext := append([]byte("v20"), append(nonce, gcmEnc...)...)

	t.Run("all tiers populated: v20 picks V20, decrypts", func(t *testing.T) {
		got, err := decryptValue(keyretriever.MasterKeys{V10: k10, V11: k11, V20: k20}, v20Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, got)
	})

	t.Run("V20 holds wrong key: v20 still picks V20 slot (not V10/V11), errors", func(t *testing.T) {
		// If the dispatcher incorrectly fell back to V10 or V11 when V20 had a wrong key, this
		// would succeed. Proves the router uses prefix-based selection, not first-usable-key.
		_, err := decryptValue(keyretriever.MasterKeys{V10: k20, V11: k20, V20: k10}, v20Ciphertext)
		require.Error(t, err)
	})

	t.Run("only V20 populated: v20 still decrypts", func(t *testing.T) {
		// The pre-#578 symmetric regression: when DPAPI/keyring failed and only V20 was retrieved,
		// v20 cookies had to still decrypt. This asserts V10 and V11 being nil doesn't block v20.
		got, err := decryptValue(keyretriever.MasterKeys{V20: k20}, v20Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, got)
	})

	t.Run("V20 slot unpopulated: v20 errors (no key to use)", func(t *testing.T) {
		_, err := decryptValue(keyretriever.MasterKeys{V10: k10, V11: k11}, v20Ciphertext)
		require.Error(t, err)
	})
}
