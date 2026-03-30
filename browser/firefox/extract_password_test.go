package firefox

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These values are from crypto/asn1pbe_test.go loginPBETestCases.
// loginPBE hex decrypts to "Hello, World!" with globalSalt = "moond4rk" * 3.
const loginPBEHex = "303b0410f8000000000000000000000000000001301506092a864886f70d010503040830313233343536370410fe968b6565149114ea688defd6683e45"

var testGlobalSalt = bytes.Repeat([]byte("moond4rk"), 3) // 24 bytes

func loginPBEBase64(t *testing.T) string {
	t.Helper()
	raw, err := hex.DecodeString(loginPBEHex)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestExtractPasswords(t *testing.T) {
	encB64 := loginPBEBase64(t)

	// Construct a logins.json with known encrypted username/password
	json := fmt.Sprintf(`{
		"logins": [
			{
				"hostname": "https://example.com",
				"formSubmitURL": "https://example.com/login",
				"encryptedUsername": "%s",
				"encryptedPassword": "%s",
				"timeCreated": 1700000000000
			}
		]
	}`, encB64, encB64)

	path := createTestJSON(t, "logins.json", json)

	got, err := extractPasswords(testGlobalSalt, path)
	require.NoError(t, err)
	require.Len(t, got, 1)

	// Both username and password decrypt to "Hello, World!"
	assert.Equal(t, "Hello, World!", got[0].Username)
	assert.Equal(t, "Hello, World!", got[0].Password)
	assert.Equal(t, "https://example.com/login", got[0].URL)
	assert.False(t, got[0].CreatedAt.IsZero())
}

func TestExtractPasswords_FormSubmitURLFallback(t *testing.T) {
	encB64 := loginPBEBase64(t)

	// When formSubmitURL is empty, should fall back to hostname
	json := fmt.Sprintf(`{
		"logins": [
			{
				"hostname": "https://fallback.com",
				"formSubmitURL": "",
				"encryptedUsername": "%s",
				"encryptedPassword": "%s",
				"timeCreated": 1700000000000
			}
		]
	}`, encB64, encB64)

	path := createTestJSON(t, "logins.json", json)

	got, err := extractPasswords(testGlobalSalt, path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "https://fallback.com", got[0].URL)
}

func TestExtractPasswords_InvalidBase64Skipped(t *testing.T) {
	// Invalid base64 in encryptedUsername — entry should be skipped
	json := `{
		"logins": [
			{
				"hostname": "https://bad.com",
				"encryptedUsername": "not-valid-base64!!!",
				"encryptedPassword": "also-bad",
				"timeCreated": 1700000000000
			}
		]
	}`

	path := createTestJSON(t, "logins.json", json)

	got, err := extractPasswords(testGlobalSalt, path)
	require.NoError(t, err)
	assert.Empty(t, got) // skipped, not error
}

func TestExtractPasswords_EmptyLogins(t *testing.T) {
	path := createTestJSON(t, "logins.json", `{"logins": []}`)

	got, err := extractPasswords(testGlobalSalt, path)
	require.NoError(t, err)
	assert.Empty(t, got)
}
