package chromium

import (
	"bytes"
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
	err := extractCreditCards(masterkey.Set{}, path, &out)
	got := collected[record.CreditCard](t, &out)
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

func TestExtractYandexCreditCards(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexCreditCardDB(t, masterKey, dataKey,
		yandexCreditCard{
			GUID:           "card-1",
			CardHolder:     "Alice Smith",
			CardTitle:      "Personal Visa",
			ExpYear:        "2030",
			ExpMonth:       "06",
			FullCardNumber: "4111111111111111",
			PinCode:        "123",
			SecretComment:  "main card",
		},
		yandexCreditCard{
			GUID:           "card-2",
			CardHolder:     "Alice Smith",
			CardTitle:      "Backup",
			ExpYear:        "2028",
			ExpMonth:       "12",
			FullCardNumber: "5555555555554444",
			PinCode:        "456",
			SecretComment:  "",
		},
	)

	var out testCollector
	err := extractYandexCreditCards(masterkey.Set{masterkey.V10: masterKey}, path, &out)
	got := collected[record.CreditCard](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byGUID := map[string]int{}
	for i, c := range got {
		byGUID[c.GUID] = i
	}

	c1 := got[byGUID["card-1"]]
	assert.Equal(t, "Alice Smith", c1.Name)
	assert.Equal(t, "Personal Visa", c1.NickName)
	assert.Equal(t, "2030", c1.ExpYear)
	assert.Equal(t, "06", c1.ExpMonth)
	assert.Equal(t, "4111111111111111", c1.Number)
	assert.Equal(t, "123", c1.CVC)
	assert.Equal(t, "main card", c1.Comment)

	c2 := got[byGUID["card-2"]]
	assert.Equal(t, "5555555555554444", c2.Number)
	assert.Equal(t, "456", c2.CVC)
	assert.Empty(t, c2.Comment)
}

func TestExtractYandexCreditCards_WrongMasterKey(t *testing.T) {
	goodKey := bytes.Repeat([]byte{0x11}, 32)
	wrongKey := bytes.Repeat([]byte{0x99}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexCreditCardDB(t, goodKey, dataKey,
		yandexCreditCard{GUID: "g1", FullCardNumber: "4111"},
	)

	var out testCollector
	require.NoError(t, extractYandexCreditCards(masterkey.Set{masterkey.V10: wrongKey}, path, &out))
	cards := collected[record.CreditCard](t, &out)
	require.Len(t, cards, 1, "the card exports without its number")
	assert.Equal(t, "g1", cards[0].GUID)
	assert.Empty(t, cards[0].Number)
	assert.Len(t, out.fails, 1)
}

func TestYandexCardAAD(t *testing.T) {
	got := yandexCardAAD("card-guid-1", nil)
	assert.Equal(t, "card-guid-1", string(got))

	got = yandexCardAAD("g", []byte("ID"))
	assert.Equal(t, "gID", string(got))
}
