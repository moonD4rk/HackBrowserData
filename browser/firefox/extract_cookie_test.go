package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMozCookieDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "cookies.sqlite", []string{mozCookiesSchema},
		insertMozCookie("session", "abc123", ".example.com", "/", 1700000000000000, 1800000000, 1, 1),
		insertMozCookie("token", "xyz789", ".new.com", "/api", 1710000000000000, 1810000000, 1, 0),
	)
}

func TestExtractCookies(t *testing.T) {
	path := setupMozCookieDB(t)

	got, err := extractCookies(path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: creation time descending
	assert.Equal(t, ".new.com", got[0].Host)
	assert.Equal(t, ".example.com", got[1].Host)

	// Verify field mapping — Firefox cookies are plaintext
	assert.Equal(t, "token", got[0].Name)
	assert.Equal(t, "xyz789", got[0].Value)
	assert.Equal(t, "/api", got[0].Path)
	assert.True(t, got[0].IsSecure)
	assert.False(t, got[0].IsHTTPOnly)
	assert.True(t, got[0].HasExpire)    // expiry > 0
	assert.True(t, got[0].IsPersistent) // expiry > 0

	assert.Equal(t, "abc123", got[1].Value)
	assert.True(t, got[1].IsHTTPOnly)
}

func TestCountCookies(t *testing.T) {
	path := setupMozCookieDB(t)

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountCookies_Empty(t *testing.T) {
	path := createTestDB(t, "cookies.sqlite", []string{mozCookiesSchema})

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
