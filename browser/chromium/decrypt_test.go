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
		name string
		key  []byte
		want []byte
	}{
		{
			name: "decrypts correctly",
			key:  testAESKey,
			want: plaintext,
		},
		{
			name: "wrong key returns error-free empty result",
			key:  []byte("wrong_key_1234!!"),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := decryptValue(tt.key, v10Ciphertext)
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
