package chromium

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

func setupCookieDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "Cookies", cookiesSchema,
		insertCookie("session", ".old.com", "/", "", 13340000000000000, 13350000000000000, 1, 1),
		insertCookie("token", ".new.com", "/api", "", 13360000000000000, 13370000000000000, 1, 0),
	)
}

func TestExtractCookies(t *testing.T) {
	path := setupCookieDB(t)

	got, err := extractCookies(keyretriever.MasterKeys{}, path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: creation time descending (newest first)
	assert.Equal(t, ".new.com", got[0].Host)
	assert.Equal(t, ".old.com", got[1].Host)

	// Verify field mapping
	assert.Equal(t, "token", got[0].Name)
	assert.Equal(t, "/api", got[0].Path)
	assert.True(t, got[0].IsSecure)
	assert.False(t, got[0].IsHTTPOnly)
	assert.True(t, got[0].HasExpire)
	assert.True(t, got[0].IsPersistent)
	assert.False(t, got[0].CreatedAt.IsZero())
	assert.True(t, got[0].ExpireAt.After(got[0].CreatedAt))
	assert.True(t, got[1].IsHTTPOnly)
}

func TestCountCookies(t *testing.T) {
	path := setupCookieDB(t)

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountCookies_Empty(t *testing.T) {
	path := createTestDB(t, "Cookies", cookiesSchema)

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestStripCookieHash(t *testing.T) {
	googleHash := sha256.Sum256([]byte(".google.com"))
	shopifyHash := sha256.Sum256([]byte(".shopify.com"))

	tests := []struct {
		name    string
		value   []byte
		hostKey string
		want    string
	}{
		{
			name:    "Chrome 130+ strips SHA256 prefix",
			value:   append(googleHash[:], []byte("GA1.3.240937927.1770097858")...),
			hostKey: ".google.com",
			want:    "GA1.3.240937927.1770097858",
		},
		{
			name:    "Chrome 130+ empty original value",
			value:   shopifyHash[:],
			hostKey: ".shopify.com",
			want:    "",
		},
		{
			name:    "older Chrome no prefix",
			value:   []byte("plain_cookie_value"),
			hostKey: ".example.com",
			want:    "plain_cookie_value",
		},
		{
			name:    "short value unchanged",
			value:   []byte("short"),
			hostKey: ".example.com",
			want:    "short",
		},
		{
			name:    "host mismatch not stripped",
			value:   append(googleHash[:], []byte("value")...),
			hostKey: ".other.com",
			want:    string(append(googleHash[:], []byte("value")...)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCookieHash(tt.value, tt.hostKey)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestStripCookieHash_NilValue(t *testing.T) {
	got := stripCookieHash(nil, ".example.com")
	assert.Nil(t, got)
}
