//go:build darwin || linux

package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
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

func TestDecryptValue_V11(t *testing.T) {
	plaintext := []byte("test_secret_value")
	testCBCIV := bytes.Repeat([]byte{0x20}, 16)
	cbcEncrypted, err := crypto.AESCBCEncrypt(testAESKey, testCBCIV, plaintext)
	require.NoError(t, err)
	v11Ciphertext := append([]byte("v11"), cbcEncrypted...)

	// v11 ciphertexts route to the V11 slot (Linux's keyring-derived kV11Key) — not V10 (peanuts).
	got, err := decryptValue(masterkey.Set{masterkey.V11: testAESKey}, v11Ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

// TestDecryptValue_V10_V11_SlotSeparation is #578 on Linux: v10 and v11 values each route to their own key.
func TestDecryptValue_V10_V11_SlotSeparation(t *testing.T) {
	v10Key := bytes.Repeat([]byte{0x10}, 16) // V10 slot key (peanuts-derived kV10Key)
	v11Key := bytes.Repeat([]byte{0x11}, 16) // V11 slot key (keyring-derived kV11Key)

	iv := bytes.Repeat([]byte{0x20}, 16) // matches crypto.chromiumCBCIV on darwin/linux
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
