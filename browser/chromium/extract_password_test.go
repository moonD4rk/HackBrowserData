package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

func setupLoginDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "Login Data", loginsSchema,
		insertLogin("https://old.com", "https://old.com/login", "alice", "", 13340000000000000),
		insertLogin("https://new.com", "https://new.com/login", "bob", "", 13360000000000000),
	)
}

func TestExtractPasswords(t *testing.T) {
	path := setupLoginDB(t)

	got, err := extractPasswords(keyretriever.MasterKeys{}, path)
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

func TestCountPasswords(t *testing.T) {
	path := setupLoginDB(t)

	count, err := countPasswords(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountPasswords_Empty(t *testing.T) {
	path := createTestDB(t, "Login Data", loginsSchema)

	count, err := countPasswords(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestExtractYandexPasswords(t *testing.T) {
	path := createTestDB(t, "Ya Passman Data", loginsSchema,
		insertLogin("https://origin.yandex.ru", "https://action.yandex.ru/submit", "user", "", 13350000000000000),
	)

	got, err := extractYandexPasswords(keyretriever.MasterKeys{}, path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "https://action.yandex.ru/submit", got[0].URL) // action_url, not origin_url
}
