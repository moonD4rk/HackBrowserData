//go:build darwin || linux

package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

// testCBCIV is the fixed IV Chrome uses on macOS/Linux (16 space bytes).
var testCBCIV = bytes.Repeat([]byte{0x20}, 16)

func TestDecryptValue_V10(t *testing.T) {
	plaintext := []byte("test_secret_value")
	encrypted, err := crypto.AES128CBCEncrypt(testAESKey, testCBCIV, plaintext)
	require.NoError(t, err)
	v10Ciphertext := append([]byte("v10"), encrypted...)

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
			wantErrMsg: "pkcs5UnPadding",
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

func TestDecryptValue_V20(t *testing.T) {
	// v20 App-Bound Encryption is not yet implemented.
	// TODO: add successful decryption cases when implemented.
	ciphertext := append([]byte("v20"), make([]byte, 32)...)
	_, err := decryptValue(nil, ciphertext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v20")
}
