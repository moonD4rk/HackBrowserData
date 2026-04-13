//go:build linux

package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKEmptyKey_MatchesChromium pins the runtime-derived kEmptyKey to
// Chromium's reference bytes in os_crypt_linux.cc.
func TestKEmptyKey_MatchesChromium(t *testing.T) {
	want := []byte{
		0xd0, 0xd0, 0xec, 0x9c, 0x7d, 0x77, 0xd4, 0x3a,
		0xc5, 0x41, 0x87, 0xfa, 0x48, 0x18, 0xd1, 0x7f,
	}
	assert.Equal(t, want, kEmptyKey)
	assert.Len(t, kEmptyKey, 16)
}

func TestDecryptChromium_EmptyKeyFallback(t *testing.T) {
	plaintext := []byte("legacy_kwallet_value")
	encrypted, err := AESCBCEncrypt(kEmptyKey, chromiumCBCIV, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v11"), encrypted...)

	wrongKey := bytes.Repeat([]byte{0xAA}, 16)
	got, err := DecryptChromium(wrongKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptChromium_ShortCiphertext(t *testing.T) {
	key := make([]byte, 16)
	_, err := DecryptChromium(key, []byte("v11short"))
	require.ErrorIs(t, err, errShortCiphertext)
}
