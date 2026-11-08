package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
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

	var out testCollector
	err := extractCreditCards(keysOf(masterkey.Set{}), path, &out)
	got := collected[record.CreditCard](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, "John Doe", got[0].Name)
	assert.Equal(t, "12", got[0].ExpMonth)
	assert.Equal(t, "2025", got[0].ExpYear)
	// Card number is empty because masterKey is nil (decrypt returns empty)
	assert.Empty(t, got[0].Number)

	assert.Equal(t, "Jane Smith", got[1].Name)
	assert.Equal(t, "6", got[1].ExpMonth)
	assert.Equal(t, "2027", got[1].ExpYear)
}
