package keychain

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NeitherOpensNorPrompts(t *testing.T) {
	kc := New(filepath.Join(t.TempDir(), "login.keychain-db"), func(context.Context) (string, error) {
		t.Fatal("building a keychain handle must not ask for the password")
		return "", nil
	})
	assert.NoError(t, kc.Err(), "a keychain that was never used has no error")
}

// An unreadable keychain fails before any prompt, once, and every query reports the same reason.
func TestOpenFailure_IsReportedOnceWithoutPrompting(t *testing.T) {
	prompts := 0
	kc := New(filepath.Join(t.TempDir(), "missing.keychain-db"), func(context.Context) (string, error) {
		prompts++
		return "secret", nil
	})

	_, err := kc.GenericPassword(context.Background(), "Chrome Safe Storage")
	require.ErrorContains(t, err, "open login keychain")
	_, err2 := kc.InternetPasswords(context.Background())
	require.Equal(t, err, err2)

	assert.Equal(t, err, kc.Err())
	assert.Zero(t, prompts)
}
