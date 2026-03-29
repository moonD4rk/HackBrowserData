package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractLocalStorage(t *testing.T) {
	dir := createTestLevelDB(t, map[string]string{
		"https://example.com\x00token":    "abc123",
		"https://example.com\x00theme":    "dark",
		"https://other.com\x00session_id": "xyz789",
		"noseparator":                     "should-be-skipped",
	})

	got, err := extractLocalStorage(dir)
	require.NoError(t, err)
	require.Len(t, got, 3) // "noseparator" entry skipped

	// Verify field mapping by collecting into a lookup
	byKey := map[string]string{}
	for _, entry := range got {
		byKey[entry.URL+"/"+entry.Key] = entry.Value
	}
	assert.Equal(t, "abc123", byKey["https://example.com/token"])
	assert.Equal(t, "dark", byKey["https://example.com/theme"])
	assert.Equal(t, "xyz789", byKey["https://other.com/session_id"])
}

func TestExtractSessionStorage(t *testing.T) {
	dir := createTestLevelDB(t, map[string]string{
		"https://example.com-token": "abc123",
		"https://example.com-user":  "alice",
	})

	got, err := extractSessionStorage(dir)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byKey := map[string]string{}
	for _, entry := range got {
		byKey[entry.Key] = entry.Value
	}
	assert.Equal(t, "abc123", byKey["token"])
	assert.Equal(t, "alice", byKey["user"])
}
