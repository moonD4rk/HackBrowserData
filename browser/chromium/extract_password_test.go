package chromium

import (
	"bytes"
	"crypto/sha1"
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
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexPasswordDB(t, masterKey, dataKey, false,
		yandexPassword{
			OriginURL: "https://old.yandex.ru", UsernameElem: "u", UsernameVal: "alice",
			PasswordElem: "p", SignonRealm: "https://old.yandex.ru", Password: "hunter2",
			DateCreated: 13340000000000000,
		},
		yandexPassword{
			OriginURL: "https://new.yandex.ru", UsernameElem: "u", UsernameVal: "bob",
			PasswordElem: "p", SignonRealm: "https://new.yandex.ru", Password: "sesame",
			DateCreated: 13360000000000000,
		},
	)

	got, err := extractYandexPasswords(keyretriever.MasterKeys{V10: masterKey}, path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Sorted newest-first on CreatedAt.
	assert.Equal(t, "https://new.yandex.ru", got[0].URL)
	assert.Equal(t, "bob", got[0].Username)
	assert.Equal(t, "sesame", got[0].Password)
	assert.Equal(t, "hunter2", got[1].Password)
}

func TestExtractYandexPasswords_MasterPasswordSkipped(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexPasswordDB(t, masterKey, dataKey, true,
		yandexPassword{
			OriginURL: "https://yandex.ru", UsernameElem: "u", UsernameVal: "alice",
			PasswordElem: "p", SignonRealm: "https://yandex.ru", Password: "hunter2",
			DateCreated: 13340000000000000,
		},
	)

	got, err := extractYandexPasswords(keyretriever.MasterKeys{V10: masterKey}, path)
	require.NoError(t, err)
	assert.Empty(t, got, "master-password profiles should be skipped in v1")
}

func TestExtractYandexPasswords_WrongMasterKey(t *testing.T) {
	goodKey := bytes.Repeat([]byte{0x11}, 32)
	wrongKey := bytes.Repeat([]byte{0x99}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexPasswordDB(t, goodKey, dataKey, false,
		yandexPassword{
			OriginURL: "https://yandex.ru", UsernameElem: "u", UsernameVal: "alice",
			PasswordElem: "p", SignonRealm: "https://yandex.ru", Password: "hunter2",
		},
	)

	// A wrong master key fails at the intermediate step, surfacing as an error
	// from the extractor.
	_, err := extractYandexPasswords(keyretriever.MasterKeys{V10: wrongKey}, path)
	require.Error(t, err)
}

func TestYandexLoginAAD_NoMasterPassword(t *testing.T) {
	got := yandexLoginAAD("https://example.com/", "user", "alice", "pass", "https://example.com/", nil)

	h := sha1.New()
	h.Write([]byte("https://example.com/\x00user\x00alice\x00pass\x00https://example.com/"))
	want := h.Sum(nil)

	assert.Equal(t, want, got)
	assert.Len(t, got, sha1.Size)
}

func TestYandexLoginAAD_WithMasterPassword(t *testing.T) {
	keyID := []byte("abc123")
	got := yandexLoginAAD("u", "e1", "v1", "e2", "r", keyID)

	require.Len(t, got, sha1.Size+len(keyID))
	assert.Equal(t, keyID, got[sha1.Size:])
}
