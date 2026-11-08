package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
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

	var out testCollector
	err := extractPasswords(keysOf(masterkey.Set{}), path, record.StoreLocal, &out)
	got := collected[record.Login](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Rows keep the database's order; the exporter sorts them.
	assert.Equal(t, "https://old.com", got[0].URL)
	assert.Equal(t, "https://new.com", got[1].URL)

	assert.Equal(t, "bob", got[1].Username)
	assert.False(t, got[1].CreatedAt.IsZero())
	// Password is empty because masterKey is nil (decrypt returns empty)
	assert.Empty(t, got[1].Password)
	assert.Equal(t, record.StoreLocal, got[1].Store)
}

// TestExtractPasswords_UndecryptableBlobStillListed keeps such a login, or a missing key would shrink the export.
func TestExtractPasswords_UndecryptableBlobStillListed(t *testing.T) {
	path := createTestDB(t, "Login Data", loginsSchema,
		insertLogin("https://enc.com", "https://enc.com/login", "carol", "763130deadbeef", 13350000000000000),
	)

	var out testCollector
	err := extractPasswords(keysOf(masterkey.Set{}), path, record.StoreAccount, &out)
	got := collected[record.Login](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 1)

	assert.Equal(t, "carol", got[0].Username)
	assert.Empty(t, got[0].Password)
	assert.Equal(t, record.StoreAccount, got[0].Store)
	assert.Len(t, out.fails, 1, "the unrecovered password is reported, which makes the extraction partial")
}
