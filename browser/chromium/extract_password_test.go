package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPasswords(t *testing.T) {
	path := createTestDB(t, "Login Data", loginsSchema,
		insertLogin("https://old.com", "https://old.com/login", "alice", "", 13340000000000000),
		insertLogin("https://new.com", "https://new.com/login", "bob", "", 13360000000000000),
	)

	got, err := extractPasswords(nil, path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: date created descending (newest first)
	assert.Equal(t, "https://new.com", got[0].URL)
	assert.Equal(t, "https://old.com", got[1].URL)

	// Verify field mapping
	assert.Equal(t, "bob", got[0].Username)
	assert.False(t, got[0].CreatedAt.IsZero())
	// Password is empty because masterKey is nil (decrypt returns empty)
	assert.Empty(t, got[0].Password)
}

func TestExtractYandexPasswords(t *testing.T) {
	path := createTestDB(t, "Ya Passman Data", loginsSchema,
		insertLogin("https://origin.yandex.ru", "https://action.yandex.ru/submit", "user", "", 13350000000000000),
	)

	got, err := extractYandexPasswords(nil, path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "https://action.yandex.ru/submit", got[0].URL) // action_url, not origin_url
}
