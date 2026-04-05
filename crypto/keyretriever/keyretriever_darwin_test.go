//go:build darwin

package keyretriever

import (
	"testing"

	"github.com/moond4rk/keychainbreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindStorageKey_Found(t *testing.T) {
	records := []keychainbreaker.GenericPassword{
		{Account: "Chrome", Password: []byte("mock-secret")},
		{Account: "Brave", Password: []byte("brave-secret")},
	}

	key, err := findStorageKey(records, "Chrome")
	require.NoError(t, err)
	assert.Equal(t, darwinParams.deriveKey([]byte("mock-secret")), key)
}

func TestFindStorageKey_NotFound(t *testing.T) {
	records := []keychainbreaker.GenericPassword{
		{Account: "Chrome", Password: []byte("mock-secret")},
	}

	key, err := findStorageKey(records, "Firefox")
	require.Error(t, err)
	assert.Nil(t, key)
	assert.ErrorIs(t, err, errStorageNotFound)
}

func TestKeychainPasswordRetriever_EmptyPassword(t *testing.T) {
	r := &KeychainPasswordRetriever{Password: ""}
	key, err := r.RetrieveKey("Chrome", "")
	require.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "keychain password not provided")
}

func TestTerminalPasswordRetriever_NonTTY(t *testing.T) {
	// In CI/test environments, stdin is not a TTY.
	// The retriever should silently return nil, nil to let the chain continue.
	r := &TerminalPasswordRetriever{}
	key, err := r.RetrieveKey("Chrome", "")
	require.NoError(t, err)
	assert.Nil(t, key)
}
