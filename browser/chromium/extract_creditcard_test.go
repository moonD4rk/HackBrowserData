package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

func setupCreditCardDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "Web Data", creditCardsSchema,
		insertCreditCard("John Doe", 12, 2025, "", "Johnny", "addr-1"),
		insertCreditCard("Jane Smith", 6, 2027, "", "", ""),
	)
}

func TestExtractCreditCards(t *testing.T) {
	path := setupCreditCardDB(t)

	got, err := extractCreditCards(keyretriever.MasterKeys{}, path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify field mapping
	assert.Equal(t, "John Doe", got[0].Name)
	assert.Equal(t, "12", got[0].ExpMonth)
	assert.Equal(t, "2025", got[0].ExpYear)
	// Card number is empty because masterKey is nil (decrypt returns empty)
	assert.Empty(t, got[0].Number)

	assert.Equal(t, "Jane Smith", got[1].Name)
	assert.Equal(t, "6", got[1].ExpMonth)
	assert.Equal(t, "2027", got[1].ExpYear)
}

func TestCountCreditCards(t *testing.T) {
	path := setupCreditCardDB(t)

	count, err := countCreditCards(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountCreditCards_Empty(t *testing.T) {
	path := createTestDB(t, "Web Data", creditCardsSchema)

	count, err := countCreditCards(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
