package chromium

import (
	"crypto/sha256"
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

func TestStripCookieHash_ChromeV24(t *testing.T) {
	host := ".google.com"
	realValue := "GA1.3.240937927.1770097858"

	// Simulate Chrome 130+ (DB v24): SHA256(host) prepended to value
	hash := sha256.Sum256([]byte(host))
	withHash := append(hash[:], []byte(realValue)...)

	got := stripCookieHash(withHash, host)
	assert.Equal(t, realValue, string(got))
}

func TestStripCookieHash_OlderChrome(t *testing.T) {
	// Pre-Chrome 130: no hash prefix, value returned unchanged
	oldValue := []byte("plain_cookie_value")
	got := stripCookieHash(oldValue, ".example.com")
	assert.Equal(t, "plain_cookie_value", string(got))
}

func TestStripCookieHash_EmptyOriginalValue(t *testing.T) {
	// Chrome 130+: cookie with empty value → only SHA256(host) remains after decryption
	host := ".shopify.com"
	hash := sha256.Sum256([]byte(host))

	got := stripCookieHash(hash[:], host)
	assert.Empty(t, got) // actual value was "", so result should be empty
}

func TestStripCookieHash_ShortValue(t *testing.T) {
	// Value shorter than 32 bytes: returned unchanged
	got := stripCookieHash([]byte("short"), ".example.com")
	assert.Equal(t, "short", string(got))
}

func TestStripCookieHash_EmptyValue(t *testing.T) {
	got := stripCookieHash(nil, ".example.com")
	assert.Nil(t, got)
}

func TestStripCookieHash_HostMismatch(t *testing.T) {
	host := ".google.com"
	realValue := "some_value"

	// Hash was computed for .google.com
	hash := sha256.Sum256([]byte(host))
	withHash := append(hash[:], []byte(realValue)...)

	// But we check against a different host — should NOT strip
	got := stripCookieHash(withHash, ".other.com")
	assert.Len(t, got, sha256.Size+len(realValue)) // unchanged
}
