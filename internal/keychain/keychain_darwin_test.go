package keychain

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NeitherOpensNorPrompts(t *testing.T) {
	loginKeychain := New(filepath.Join(t.TempDir(), "login.keychain-db"), func(context.Context) (string, error) {
		t.Fatal("building a keychain handle must not ask for the password")
		return "", nil
	})
	assert.NoError(t, loginKeychain.Err(), "a keychain that was never used has no error")
}

// TestOpenFailure_IsReportedOnceWithoutPrompting fails an unreadable keychain once, before any prompt, for every query.
func TestOpenFailure_IsReportedOnceWithoutPrompting(t *testing.T) {
	prompts := 0
	loginKeychain := New(filepath.Join(t.TempDir(), "missing.keychain-db"), func(context.Context) (string, error) {
		prompts++
		return "secret", nil
	})

	_, err := loginKeychain.GenericPassword(context.Background(), "Chrome Safe Storage")
	require.ErrorContains(t, err, "open login keychain")
	_, secondErr := loginKeychain.InternetPasswords(context.Background())
	require.Equal(t, err, secondErr)

	assert.Equal(t, err, loginKeychain.Err())
	assert.Zero(t, prompts)
}
