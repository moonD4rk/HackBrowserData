package keyretriever

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRetriever struct {
	key []byte
	err error
}

func (m *mockRetriever) RetrieveKey(_ Hints) ([]byte, error) {
	return m.key, m.err
}

func TestChainRetriever_FirstSuccess(t *testing.T) {
	chain := NewChain(
		&mockRetriever{key: []byte("first-key")},
		&mockRetriever{key: []byte("second-key")},
	)
	key, err := chain.RetrieveKey(Hints{KeychainLabel: "Chrome"})
	require.NoError(t, err)
	assert.Equal(t, []byte("first-key"), key)
}

func TestChainRetriever_FallbackOnError(t *testing.T) {
	chain := NewChain(
		&mockRetriever{err: errors.New("first failed")},
		&mockRetriever{key: []byte("fallback-key")},
	)
	key, err := chain.RetrieveKey(Hints{KeychainLabel: "Chrome"})
	require.NoError(t, err)
	assert.Equal(t, []byte("fallback-key"), key)
}

func TestChainRetriever_AllFail(t *testing.T) {
	chain := NewChain(
		&mockRetriever{err: errors.New("first failed")},
		&mockRetriever{err: errors.New("second failed")},
	)
	key, err := chain.RetrieveKey(Hints{KeychainLabel: "Chrome"})
	require.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "all retrievers failed")
	assert.Contains(t, err.Error(), "first failed")
	assert.Contains(t, err.Error(), "second failed")
}

func TestChainRetriever_SkipEmptyKey(t *testing.T) {
	// First returns nil key without error — should skip to next
	chain := NewChain(
		&mockRetriever{key: nil, err: nil},
		&mockRetriever{key: []byte("real-key")},
	)
	key, err := chain.RetrieveKey(Hints{KeychainLabel: "Chrome"})
	require.NoError(t, err)
	assert.Equal(t, []byte("real-key"), key)
}

func TestChainRetriever_Empty(t *testing.T) {
	chain := NewChain()
	key, err := chain.RetrieveKey(Hints{KeychainLabel: "Chrome"})
	require.Error(t, err)
	assert.Nil(t, key)
}
