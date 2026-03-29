//go:build darwin || linux

package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

func TestDecryptValue_V10RoundTrip(t *testing.T) {
	// Construct a v10 ciphertext using the same method Chrome uses on macOS/Linux:
	// AES-128-CBC with a fixed IV of 16 space bytes (0x20).
	key := []byte("0123456789abcdef") // 16-byte AES key
	iv := []byte("                ")  // 16 space bytes, same as Chrome
	plaintext := []byte("my_secret_cookie_value")

	encrypted, err := crypto.AES128CBCEncrypt(key, iv, plaintext)
	require.NoError(t, err)

	// Prepend "v10" prefix, just like Chrome stores it
	ciphertext := append([]byte("v10"), encrypted...)

	// decryptValue should detect v10 and decrypt correctly
	got, err := decryptValue(key, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptValue_Empty(t *testing.T) {
	got, err := decryptValue(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDecryptValue_V20Unsupported(t *testing.T) {
	ciphertext := append([]byte("v20"), make([]byte, 32)...)
	_, err := decryptValue(nil, ciphertext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v20")
}

func TestDecryptValue_InvalidData(t *testing.T) {
	// Non-v10, non-v20 prefix — goes to DPAPI path which fails on macOS/Linux
	_, err := decryptValue(nil, []byte{0x01, 0x02, 0x03, 0x04})
	require.Error(t, err)
}
