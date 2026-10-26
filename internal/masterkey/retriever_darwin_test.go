//go:build darwin

package masterkey

import (
	"context"
	"fmt"
	"testing"

	"github.com/moond4rk/keychainbreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
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

type fakeKeychain struct {
	secrets map[string][]byte
	err     error
	asked   []string
}

func (k *fakeKeychain) GenericPassword(_ context.Context, account string) ([]byte, error) {
	k.asked = append(k.asked, account)
	if k.err != nil {
		return nil, k.err
	}
	secret, ok := k.secrets[account]
	if !ok {
		return nil, fmt.Errorf("%q: not found", account)
	}
	return secret, nil
}

func TestKeychainRetriever(t *testing.T) {
	ctx := context.Background()
	loc := func(kc LoginKeychain) Locator { return Locator{KeychainLabel: "Chrome", Keychain: kc} }

	t.Run("no keychain steps aside", func(t *testing.T) {
		_, err := keychainRetriever{}.Retrieve(ctx, Locator{KeychainLabel: "Chrome"})
		require.ErrorIs(t, err, ErrNotApplicable)
	})

	t.Run("locked keychain steps aside", func(t *testing.T) {
		kc := &fakeKeychain{err: fmt.Errorf("%w: wrong password", keychain.ErrLocked)}
		_, err := keychainRetriever{}.Retrieve(ctx, loc(kc))
		require.ErrorIs(t, err, ErrNotApplicable)
	})

	t.Run("secret derives the v10 key", func(t *testing.T) {
		kc := &fakeKeychain{secrets: map[string][]byte{"Chrome": []byte("mock-secret")}}
		key, err := keychainRetriever{}.Retrieve(ctx, loc(kc))
		require.NoError(t, err)
		assert.Equal(t, darwinParams.deriveKey([]byte("mock-secret")), key)
		assert.Equal(t, []string{"Chrome"}, kc.asked)
	})

	t.Run("missing label is an error", func(t *testing.T) {
		_, err := keychainRetriever{}.Retrieve(ctx, loc(&fakeKeychain{secrets: map[string][]byte{}}))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNotApplicable)
	})
}

func TestPlatformRetrievers_Darwin(t *testing.T) {
	require.Len(t, platformRetrievers, 3)
	assert.IsType(t, &gcoredumpRetriever{}, platformRetrievers[0])
	assert.IsType(t, keychainRetriever{}, platformRetrievers[1])
	assert.IsType(t, &securityRetriever{}, platformRetrievers[2])
	for _, r := range platformRetrievers {
		assert.Equal(t, V10, r.Version())
	}
}
