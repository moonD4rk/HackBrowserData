package chromium

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
)

func TestDetectVersion(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext []byte
		want       cipherVersion
	}{
		{"v10 prefix", []byte("v10" + "encrypted_data"), cipherV10},
		{"v11 prefix", []byte("v11" + "encrypted_data"), cipherV11},
		{"v12 prefix", []byte("v12" + "encrypted_data"), cipherV12},
		{"v20 prefix", []byte("v20" + "encrypted_data"), cipherV20},
		{"no prefix (DPAPI)", []byte{0x01, 0x00, 0x00, 0x00}, cipherDPAPI},
		{"short input", []byte{0x01, 0x02}, cipherDPAPI},
		{"empty input", []byte{}, cipherDPAPI},
		{"nil input", nil, cipherDPAPI},
		{"unknown prefix", []byte("xyz_data"), cipherDPAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectVersion(tt.ciphertext))
		})
	}
}

// --- Cross-OS Chromium v10/v11 decryption ---
// decryptGCM / decryptCBC are platform-neutral, so these run on every
// GOOS and prove a key dumped on one platform decrypts that platform's data anywhere.

var (
	// cbcKey is the 16-byte AES-128-CBC tier (macOS/Linux v10/v11); gcmKey the 32-byte AES-256-GCM tier (Windows v10 / v20).
	cbcKey   = bytes.Repeat([]byte("moond4rk"), 2)
	gcmKey   = bytes.Repeat([]byte("moond4rk"), 4)
	gcmNonce = cbcKey[:12]
)

func TestDecryptGCM_CrossPlatform(t *testing.T) {
	plaintext := []byte("windows_v10_value")
	gcm, err := crypto.AESGCMEncrypt(gcmKey, gcmNonce, plaintext)
	require.NoError(t, err)

	ciphertext := append([]byte("v10"), append(gcmNonce, gcm...)...)
	got, err := decryptGCM(gcmKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptCBC_CrossPlatform(t *testing.T) {
	plaintext := []byte("posix_v10_value")
	enc, err := crypto.AESCBCEncrypt(cbcKey, chromiumCBCIV, plaintext)
	require.NoError(t, err)

	ciphertext := append([]byte("v10"), enc...)
	got, err := decryptCBC(cbcKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

// TestKEmptyKey_MatchesChromium pins the runtime-derived kEmptyKey to Chromium's
// reference bytes in os_crypt_linux.cc; now cross-platform since kEmptyKey is
// defined for every GOOS.
func TestKEmptyKey_MatchesChromium(t *testing.T) {
	want := []byte{
		0xd0, 0xd0, 0xec, 0x9c, 0x7d, 0x77, 0xd4, 0x3a,
		0xc5, 0x41, 0x87, 0xfa, 0x48, 0x18, 0xd1, 0x7f,
	}
	assert.Equal(t, want, kEmptyKey)
	assert.Len(t, kEmptyKey, 16)
}

func TestDecryptCBC_EmptyKeyFallback(t *testing.T) {
	plaintext := []byte("legacy_kwallet_value")
	encrypted, err := crypto.AESCBCEncrypt(kEmptyKey, chromiumCBCIV, plaintext)
	require.NoError(t, err)
	ciphertext := append([]byte("v11"), encrypted...)

	wrongKey := bytes.Repeat([]byte{0xAA}, 16)
	got, err := decryptCBC(wrongKey, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecrypt_ShortCiphertext(t *testing.T) {
	// GCM minimum is prefix(3)+nonce(12) = 15 bytes.
	_, err := decryptGCM(gcmKey, []byte("v10nonce11"))
	require.ErrorIs(t, err, errShortCiphertext)

	// CBC minimum is prefix(3)+block(16) = 19 bytes.
	_, err = decryptCBC(cbcKey, []byte("v11short"))
	require.ErrorIs(t, err, errShortCiphertext)
}
