//go:build linux

package keyretriever

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackRetriever(t *testing.T) {
	r := &FallbackRetriever{}

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

	// "peanuts" is a fixed fallback password, so the result should be
	// the same regardless of storage name or number of calls.
	key2, err := r.RetrieveKey("Brave", "")
	require.NoError(t, err)
	assert.Equal(t, key, key2, "fallback key should be the same for any storage")
}

func TestDefaultRetriever_Linux(t *testing.T) {
	r := DefaultRetriever()
	chain, ok := r.(*ChainRetriever)
	require.True(t, ok, "DefaultRetriever should return a *ChainRetriever")

	assert.Len(t, chain.retrievers, 2, "chain should have 2 retrievers")
	assert.IsType(t, &DBusRetriever{}, chain.retrievers[0], "first retriever should be DBusRetriever")
	assert.IsType(t, &FallbackRetriever{}, chain.retrievers[1], "second retriever should be FallbackRetriever")
}
