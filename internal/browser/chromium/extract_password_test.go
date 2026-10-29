package chromium

import (
	"bytes"
	"crypto/sha1"
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
	err := extractPasswords(masterkey.Set{}, path, record.StoreLocal, &out)
	got := collected[record.Login](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Rows keep the database's order; the exporter sorts them.
	assert.Equal(t, "https://old.com", got[0].URL)
	assert.Equal(t, "https://new.com", got[1].URL)

	// Verify field mapping
	assert.Equal(t, "bob", got[1].Username)
	assert.False(t, got[1].CreatedAt.IsZero())
	// Password is empty because masterKey is nil (decrypt returns empty)
	assert.Empty(t, got[1].Password)
	assert.Equal(t, record.StoreLocal, got[1].Store)
}

// A credential whose blob cannot be decrypted must still be reported, otherwise a missing
// master key would silently shrink the export instead of yielding empty passwords.
func TestExtractPasswords_UndecryptableBlobStillListed(t *testing.T) {
	path := createTestDB(t, "Login Data", loginsSchema,
		insertLogin("https://enc.com", "https://enc.com/login", "carol", "763130deadbeef", 13350000000000000),
	)

	var out testCollector
	err := extractPasswords(masterkey.Set{}, path, record.StoreAccount, &out)
	got := collected[record.Login](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 1)

	assert.Equal(t, "carol", got[0].Username)
	assert.Empty(t, got[0].Password)
	assert.Equal(t, record.StoreAccount, got[0].Store)
	assert.Len(t, out.fails, 1, "the unrecovered password is reported, which makes the extraction partial")
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

	var out testCollector
	err := extractYandexPasswords(masterkey.Set{masterkey.V10: masterKey}, path, &out)
	got := collected[record.Login](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Rows keep the database's order; the exporter sorts them.
	assert.Equal(t, "https://new.yandex.ru", got[1].URL)
	assert.Equal(t, "bob", got[1].Username)
	assert.Equal(t, "sesame", got[1].Password)
	assert.Equal(t, "hunter2", got[0].Password)
}

func TestExtractYandexPasswords_MasterPasswordKeepsTheLogins(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexPasswordDB(t, masterKey, dataKey, true,
		yandexPassword{
			OriginURL: "https://yandex.ru", UsernameElem: "u", UsernameVal: "alice",
			PasswordElem: "p", SignonRealm: "https://yandex.ru", Password: "hunter2",
			DateCreated: 13340000000000000,
		},
	)

	var out testCollector
	require.NoError(t, extractYandexPasswords(masterkey.Set{masterkey.V10: masterKey}, path, &out))
	logins := collected[record.Login](t, &out)
	require.Len(t, logins, 1)
	assert.Equal(t, "alice", logins[0].Username)
	assert.Empty(t, logins[0].Password)
	require.Len(t, out.fails, 1)
	assert.ErrorIs(t, out.fails[0], errYandexMasterPasswordSet)
}

func TestExtractYandexPasswords_WithoutAKeyKeepsTheLogins(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)
	path := setupYandexPasswordDB(t, masterKey, dataKey, false,
		yandexPassword{OriginURL: "https://a.yandex.ru", UsernameVal: "alice", Password: "hunter2"},
		yandexPassword{OriginURL: "https://b.yandex.ru", UsernameVal: "bob", Password: "hunter3"},
	)

	var out testCollector
	require.NoError(t, extractYandexPasswords(nil, path, &out))
	logins := collected[record.Login](t, &out)
	require.Len(t, logins, 2)
	for _, l := range logins {
		assert.Empty(t, l.Password)
	}
	assert.Len(t, out.fails, 2, "each login without its password is a record-level problem")
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

	// A wrong master key fails at the intermediate step, so the login exports without its password.
	var out testCollector
	require.NoError(t, extractYandexPasswords(masterkey.Set{masterkey.V10: wrongKey}, path, &out))
	logins := collected[record.Login](t, &out)
	require.Len(t, logins, 1)
	assert.Empty(t, logins[0].Password)
	assert.Len(t, out.fails, 1)
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
