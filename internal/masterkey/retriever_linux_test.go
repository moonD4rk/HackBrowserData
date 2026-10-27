//go:build linux

package masterkey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPosixRetriever(t *testing.T) {
	r := posixRetriever{}

	key, err := r.Retrieve(context.Background(), Locator{KeychainLabel: "Chrome"})
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

	// "peanuts" is a hardcoded password, so the result should be the same regardless of the locator
	// or number of calls.
	key2, err := r.Retrieve(context.Background(), Locator{KeychainLabel: "Brave"})
	require.NoError(t, err)
	assert.Equal(t, key, key2, "kV10Key should be constant across any storage label")
}

// TestPosixRetriever_MatchesChromiumKV10Key pins posixRetriever's output to Chromium's kV10Key
// reference bytes (PBKDF2-HMAC-SHA1 of "peanuts" with "saltysalt", 1 iteration, 16 bytes).
func TestPosixRetriever_MatchesChromiumKV10Key(t *testing.T) {
	want := []byte{
		0xfd, 0x62, 0x1f, 0xe5, 0xa2, 0xb4, 0x02, 0x53,
		0x9d, 0xfa, 0x14, 0x7c, 0xa9, 0x27, 0x27, 0x78,
	}
	key, err := posixRetriever{}.Retrieve(context.Background(), Locator{})
	require.NoError(t, err)
	assert.Equal(t, want, key)
}

func TestPlatformRetrievers_Linux(t *testing.T) {
	require.Len(t, platformRetrievers, 2)
	assert.IsType(t, posixRetriever{}, platformRetrievers[0], "v10 is the peanuts kV10Key")
	assert.IsType(t, dbusRetriever{}, platformRetrievers[1], "v11 is the D-Bus keyring kV11Key")
	assert.Equal(t, V10, platformRetrievers[0].Version())
	assert.Equal(t, V11, platformRetrievers[1].Version())
}
