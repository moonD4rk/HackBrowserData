package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

var testAESKey = []byte("0123456789abcdef")

func TestDecryptValue_V10(t *testing.T) {
	plaintext := []byte("test_secret_value")
	testCBCIV := bytes.Repeat([]byte{0x20}, 16)
	cbcEncrypted, err := crypto.AESCBCEncrypt(testAESKey, testCBCIV, plaintext)
	require.NoError(t, err)
	v10Ciphertext := append([]byte("v10"), cbcEncrypted...)

	tests := []struct {
		name       string
		key        []byte
		want       []byte
		wantErrMsg string // empty = no error expected
	}{
		{
			name: "decrypts correctly",
			key:  testAESKey,
			want: plaintext,
		},
		{
			name:       "wrong key returns padding error",
			key:        []byte("wrong_key_1234!!"),
			wantErrMsg: "invalid PKCS5 padding",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := decryptValue(masterkey.Set{masterkey.V10: testCase.key}, v10Ciphertext)
			if testCase.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.wantErrMsg)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
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

// TestDecryptValue_V10_V11_SlotSeparation is #578 on Linux: v10 and v11 values each route to their own key.
func TestDecryptValue_V10_V11_SlotSeparation(t *testing.T) {
	v10Key := bytes.Repeat([]byte{0x10}, 16) // V10 slot key (peanuts-derived kV10Key)
	v11Key := bytes.Repeat([]byte{0x11}, 16) // V11 slot key (keyring-derived kV11Key)

	iv := bytes.Repeat([]byte{0x20}, 16)
	v10plain := []byte("password-from-v10-era")
	v11plain := []byte("password-from-v11-era")

	v10Enc, err := crypto.AESCBCEncrypt(v10Key, iv, v10plain)
	require.NoError(t, err)
	v10Ciphertext := append([]byte("v10"), v10Enc...)

	v11Enc, err := crypto.AESCBCEncrypt(v11Key, iv, v11plain)
	require.NoError(t, err)
	v11Ciphertext := append([]byte("v11"), v11Enc...)

	masterKeys := masterkey.Set{masterkey.V10: v10Key, masterkey.V11: v11Key}

	t.Run("v10 ciphertext decrypts via V10 slot", func(t *testing.T) {
		got, err := decryptValue(masterKeys, v10Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, v10plain, got)
	})

	t.Run("v11 ciphertext decrypts via V11 slot", func(t *testing.T) {
		got, err := decryptValue(masterKeys, v11Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, v11plain, got)
	})

	t.Run("swapped keys fail both directions", func(t *testing.T) {
		swapped := masterkey.Set{masterkey.V10: v11Key, masterkey.V11: v10Key}
		_, err := decryptValue(swapped, v10Ciphertext)
		require.Error(t, err, "v10 with V11's key must fail")
		_, err = decryptValue(swapped, v11Ciphertext)
		require.Error(t, err, "v11 with V10's key must fail")
	})
}

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

func TestDecryptValue_V20_ShortCiphertext(t *testing.T) {
	// Missing nonce (prefix only) must error, not panic.
	_, err := decryptValue(masterkey.Set{masterkey.V20: testAESKey}, []byte("v20"))
	require.Error(t, err)
}

func TestDetectVersion(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext []byte
		want       masterkey.Version
	}{
		{"v10 prefix", []byte("v10" + "encrypted_data"), masterkey.V10},
		{"v11 prefix", []byte("v11" + "encrypted_data"), masterkey.V11},
		{"v12 prefix", []byte("v12" + "encrypted_data"), versionV12},
		{"v20 prefix", []byte("v20" + "encrypted_data"), masterkey.V20},
		{"no prefix (DPAPI)", []byte{0x01, 0x00, 0x00, 0x00}, unversioned},
		{"short input", []byte{0x01, 0x02}, unversioned},
		{"empty input", []byte{}, unversioned},
		{"nil input", nil, unversioned},
		{"unknown prefix", []byte("xyz_data"), unversioned},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, detectVersion(testCase.ciphertext))
		})
	}
}

// TestEmptyPasswordKey_MatchesChromium pins emptyPasswordKey to the reference bytes in os_crypt_linux.cc.
func TestEmptyPasswordKey_MatchesChromium(t *testing.T) {
	want := []byte{
		0xd0, 0xd0, 0xec, 0x9c, 0x7d, 0x77, 0xd4, 0x3a,
		0xc5, 0x41, 0x87, 0xfa, 0x48, 0x18, 0xd1, 0x7f,
	}
	assert.Equal(t, want, emptyPasswordKey)
}

func TestDecryptCBC_EmptyKeyFallback(t *testing.T) {
	plaintext := []byte("legacy_kwallet_value")
	encrypted, err := crypto.AESCBCEncrypt(emptyPasswordKey, cbcIV, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v11"), encrypted...)

	wrongKey := bytes.Repeat([]byte{0xAA}, 16)
	got, err := decryptCBC(wrongKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecrypt_ShortCiphertext(t *testing.T) {
	cbcKey := bytes.Repeat([]byte("moond4rk"), 2)
	gcmKey := bytes.Repeat([]byte("moond4rk"), 4)
	// GCM minimum is prefix(3)+nonce(12) = 15 bytes.
	_, err := decryptGCM(gcmKey, []byte("v10nonce11"))
	require.ErrorIs(t, err, errShortCiphertext)

	// CBC minimum is prefix(3)+block(16) = 19 bytes.
	_, err = decryptCBC(cbcKey, []byte("v11short"))
	require.ErrorIs(t, err, errShortCiphertext)
}

func TestMasterKeysFuncDecrypt_EmptyValueFetchesNoKey(t *testing.T) {
	fetched := false
	keys := masterKeysFunc(func() masterkey.Set {
		fetched = true
		return nil
	})
	plaintext, err := keys.decrypt(nil)
	require.NoError(t, err)
	assert.Nil(t, plaintext)
	assert.False(t, fetched, "an empty value must not trigger key retrieval")
}
