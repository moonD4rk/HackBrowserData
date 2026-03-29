package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractCookies(t *testing.T) {
	path := createTestDB(t, "Cookies", cookiesSchema,
		insertCookie("session", ".old.com", "/", "", 13340000000000000, 13350000000000000, 1, 1),
		insertCookie("token", ".new.com", "/api", "", 13360000000000000, 13370000000000000, 1, 0),
	)

	got, err := extractCookies(nil, path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: creation time descending (newest first)
	assert.Equal(t, ".new.com", got[0].Host)
	assert.Equal(t, ".old.com", got[1].Host)

	// Verify field mapping
	assert.Equal(t, "token", got[0].Name)
	assert.Equal(t, "/api", got[0].Path)
	assert.True(t, got[0].IsSecure)
	assert.False(t, got[0].IsHTTPOnly) // httpOnly=0
	assert.False(t, got[0].CreatedAt.IsZero())
	assert.False(t, got[0].ExpireAt.IsZero())
	assert.True(t, got[0].ExpireAt.After(got[0].CreatedAt))

	// Verify second cookie flags
	assert.True(t, got[1].IsHTTPOnly) // httpOnly=1
}
