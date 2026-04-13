//go:build darwin || linux

package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
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
			got, err := decryptValue(tt.key, v10Ciphertext)
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

	got, err := decryptValue(testAESKey, v11Ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptValue_V20(t *testing.T) {
	// v20 App-Bound Encryption is not yet implemented.
	// TODO: add successful decryption cases when implemented.
	ciphertext := append([]byte("v20"), make([]byte, 32)...)
	_, err := decryptValue(nil, ciphertext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v20")
}
