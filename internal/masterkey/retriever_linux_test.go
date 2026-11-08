//go:build linux

package masterkey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPosixRetriever(t *testing.T) {
	retriever := posixRetriever{}

	key, err := retriever.Retrieve(context.Background(), Locator{SecretLabel: "Chrome"})
	require.NoError(t, err)
	assert.Equal(t, linuxParams.deriveKey([]byte("peanuts")), key)
	assert.Len(t, key, linuxParams.keySize)

	allZero := true
	for _, keyByte := range key {
		if keyByte != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero, "derived key should not be all zeros")

	// The password is fixed, so the key is the same for any locator and on every call.
	braveKey, err := retriever.Retrieve(context.Background(), Locator{SecretLabel: "Brave"})
	require.NoError(t, err)
	assert.Equal(t, key, braveKey, "kV10Key should be constant across any storage label")
}

// TestPosixRetriever_MatchesChromiumKV10Key pins the output to Chromium's kV10Key reference bytes.
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
