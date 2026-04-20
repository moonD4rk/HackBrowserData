//go:build linux

package keyretriever

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPosixRetriever(t *testing.T) {
	r := &PosixRetriever{}

	key, err := r.RetrieveKey("Chrome", "")
	require.NoError(t, err)
	assert.Equal(t, linuxParams.deriveKey([]byte("peanuts")), key)
	assert.Len(t, key, linuxParams.keySize)

	// The key should not be all zeros.
	allZero := true
	for _, b := range key {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero, "derived key should not be all zeros")

	// "peanuts" is a hardcoded password, so the result should be the same regardless of storage
	// name or number of calls.
	key2, err := r.RetrieveKey("Brave", "")
	require.NoError(t, err)
	assert.Equal(t, key, key2, "kV10Key should be constant across any storage label")
}

// TestPosixRetriever_MatchesChromiumKV10Key pins PosixRetriever's output to Chromium's kV10Key
// reference bytes (PBKDF2-HMAC-SHA1 of "peanuts" with "saltysalt", 1 iteration, 16 bytes).
func TestPosixRetriever_MatchesChromiumKV10Key(t *testing.T) {
	want := []byte{
		0xfd, 0x62, 0x1f, 0xe5, 0xa2, 0xb4, 0x02, 0x53,
		0x9d, 0xfa, 0x14, 0x7c, 0xa9, 0x27, 0x27, 0x78,
	}
	r := &PosixRetriever{}
	key, err := r.RetrieveKey("", "")
	require.NoError(t, err)
	assert.Equal(t, want, key)
}

func TestDefaultRetrievers_Linux(t *testing.T) {
	r := DefaultRetrievers()

	// V10 slot: peanuts-derived kV10Key — PosixRetriever.
	assert.IsType(t, &PosixRetriever{}, r.V10, "V10 slot should hold PosixRetriever (peanuts kV10Key)")

	// V11 slot: D-Bus keyring kV11Key — DBusRetriever.
	assert.IsType(t, &DBusRetriever{}, r.V11, "V11 slot should hold DBusRetriever (keyring kV11Key)")

	// V20 slot: ABE is Windows-only, nil on Linux.
	assert.Nil(t, r.V20, "V20 slot must stay nil on Linux")

	// Smoke: both populated slots must actually retrieve (PosixRetriever always succeeds; DBus may
	// fail in test env, which is fine — we only want to confirm the wiring, not real keys).
	require.NotNil(t, r.V10)
	require.NotNil(t, r.V11)
}
