//go:build darwin || linux

package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decryptValue(keyretriever.MasterKeys{V10: tt.key}, v10Ciphertext)
			if tt.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecryptValue_V11(t *testing.T) {
	plaintext := []byte("test_secret_value")
	testCBCIV := bytes.Repeat([]byte{0x20}, 16)
	cbcEncrypted, err := crypto.AESCBCEncrypt(testAESKey, testCBCIV, plaintext)
	require.NoError(t, err)
	v11Ciphertext := append([]byte("v11"), cbcEncrypted...)

	// v11 ciphertexts route to the V11 slot (Linux's keyring-derived kV11Key) — not V10 (peanuts).
	got, err := decryptValue(keyretriever.MasterKeys{V11: testAESKey}, v11Ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

// TestDecryptValue_V10_V11_SlotSeparation is the Linux analog of the #578 regression test: a
// profile carrying both v10 (peanuts) and v11 (keyring) ciphertexts must route each prefix to
// its own slot, not share a single key. Build-tag scoped to darwin/linux because v10/v11 use
// AES-CBC on these platforms; Windows uses AES-GCM for v10 and is covered separately by
// decrypt_windows_test.go.
func TestDecryptValue_V10_V11_SlotSeparation(t *testing.T) {
	k10 := bytes.Repeat([]byte{0x10}, 16) // V10 slot key (peanuts-derived kV10Key)
	k11 := bytes.Repeat([]byte{0x11}, 16) // V11 slot key (keyring-derived kV11Key)

	iv := bytes.Repeat([]byte{0x20}, 16) // matches crypto.chromiumCBCIV on darwin/linux
	v10plain := []byte("password-from-v10-era")
	v11plain := []byte("password-from-v11-era")

	v10Enc, err := crypto.AESCBCEncrypt(k10, iv, v10plain)
	require.NoError(t, err)
	v10Ciphertext := append([]byte("v10"), v10Enc...)

	v11Enc, err := crypto.AESCBCEncrypt(k11, iv, v11plain)
	require.NoError(t, err)
	v11Ciphertext := append([]byte("v11"), v11Enc...)

	keys := keyretriever.MasterKeys{V10: k10, V11: k11}

	t.Run("v10 ciphertext decrypts via V10 slot", func(t *testing.T) {
		got, err := decryptValue(keys, v10Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, v10plain, got)
	})

	t.Run("v11 ciphertext decrypts via V11 slot", func(t *testing.T) {
		got, err := decryptValue(keys, v11Ciphertext)
		require.NoError(t, err)
		assert.Equal(t, v11plain, got)
	})

	t.Run("swapped keys fail both directions", func(t *testing.T) {
		swapped := keyretriever.MasterKeys{V10: k11, V11: k10}
		_, err := decryptValue(swapped, v10Ciphertext)
		require.Error(t, err, "v10 with V11's key must fail")
		_, err = decryptValue(swapped, v11Ciphertext)
		require.Error(t, err, "v11 with V10's key must fail")
	})
}
