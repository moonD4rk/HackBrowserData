package chromium

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func encryptAESGCM(t *testing.T, key, nonce, plaintext, aad []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	return aead.Seal(nil, nonce, plaintext, aad)
}

// testPlaintextPayloadLen is 96 (blob) - 12 (nonce) - 16 (tag) = 68 bytes of payload before sealing.
const testPlaintextPayloadLen = yandexIntermediateKeyBlobLength - gcmNonceSize - 16

func buildLocalEncryptorBlob(t *testing.T, masterKey, dataKey []byte) []byte {
	t.Helper()
	nonce := bytes.Repeat([]byte{0xAB}, gcmNonceSize)
	plaintext := append([]byte{}, yandexSignature...)
	plaintext = append(plaintext, dataKey...)
	plaintext = append(plaintext, make([]byte, testPlaintextPayloadLen-len(plaintext))...)
	ciphertext := encryptAESGCM(t, masterKey, nonce, plaintext, nil)
	if len(ciphertext) != yandexIntermediateKeyBlobLength-gcmNonceSize {
		t.Fatalf("unexpected ciphertext len: got %d want %d", len(ciphertext), yandexIntermediateKeyBlobLength-gcmNonceSize)
	}
	blob := []byte{0x01, 0x02, 0x03, 0x04} // arbitrary protobuf preamble
	blob = append(blob, localEncryptorPrefix...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)
	blob = append(blob, 0xFF, 0xFE) // trailing junk should be ignored
	return blob
}

func TestDecryptYandexIntermediateKey_RoundTrip(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, yandexDataKeyLength)
	blob := buildLocalEncryptorBlob(t, masterKey, dataKey)

	got, err := decryptYandexIntermediateKey(masterKey, blob)
	if err != nil {
		t.Fatalf("DecryptYandexIntermediateKey: %v", err)
	}
	if !bytes.Equal(got, dataKey) {
		t.Errorf("key mismatch: got %x want %x", got, dataKey)
	}
}

func TestDecryptYandexIntermediateKey_MissingMarker(t *testing.T) {
	_, err := decryptYandexIntermediateKey(bytes.Repeat([]byte{0x11}, 32), []byte("no marker here"))
	if !errors.Is(err, errYandexMarkerNotFound) {
		t.Fatalf("expected errYandexMarkerNotFound, got %v", err)
	}
}

func TestDecryptYandexIntermediateKey_Truncated(t *testing.T) {
	blob := append([]byte{0x00, 0x00}, localEncryptorPrefix...)
	blob = append(blob, bytes.Repeat([]byte{0x55}, yandexIntermediateKeyBlobLength-1)...)
	_, err := decryptYandexIntermediateKey(bytes.Repeat([]byte{0x11}, 32), blob)
	if !errors.Is(err, errYandexBlobShort) {
		t.Fatalf("expected errYandexBlobShort, got %v", err)
	}
}

func TestDecryptYandexIntermediateKey_BadSignature(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	nonce := bytes.Repeat([]byte{0xAB}, gcmNonceSize)
	plaintext := append([]byte{0xDE, 0xAD, 0xBE, 0xEF}, bytes.Repeat([]byte{0x22}, yandexDataKeyLength)...)
	plaintext = append(plaintext, make([]byte, testPlaintextPayloadLen-len(plaintext))...)
	ciphertext := encryptAESGCM(t, masterKey, nonce, plaintext, nil)
	blob := append([]byte{}, localEncryptorPrefix...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	_, err := decryptYandexIntermediateKey(masterKey, blob)
	if !errors.Is(err, errYandexBadSignature) {
		t.Fatalf("expected errYandexBadSignature, got %v", err)
	}
}

func TestDecryptYandexIntermediateKey_TrailingDataIgnored(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	nonce := bytes.Repeat([]byte{0xAB}, gcmNonceSize)
	plaintext := append([]byte{}, yandexSignature...)
	plaintext = append(plaintext, bytes.Repeat([]byte{0x22}, 16)...)
	plaintext = append(plaintext, make([]byte, testPlaintextPayloadLen-len(plaintext))...)
	ciphertext := encryptAESGCM(t, masterKey, nonce, plaintext, nil)
	blob := append([]byte{}, localEncryptorPrefix...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	got, err := decryptYandexIntermediateKey(masterKey, blob)
	if err != nil {
		t.Fatalf("DecryptYandexIntermediateKey: %v", err)
	}
	want := bytes.Repeat([]byte{0x22}, 16)
	want = append(want, make([]byte, 16)...)
	if !bytes.Equal(got, want) {
		t.Errorf("key mismatch: got %x want %x", got, want)
	}
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
	err := extractYandexPasswords(keysOf(masterkey.Set{masterkey.V10: masterKey}), path, &out)
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
	require.NoError(t, extractYandexPasswords(keysOf(masterkey.Set{masterkey.V10: masterKey}), path, &out))
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
	require.NoError(t, extractYandexPasswords(keysOf(nil), path, &out))
	logins := collected[record.Login](t, &out)
	require.Len(t, logins, 2)
	for _, login := range logins {
		assert.Empty(t, login.Password)
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
	require.NoError(t, extractYandexPasswords(keysOf(masterkey.Set{masterkey.V10: wrongKey}), path, &out))
	logins := collected[record.Login](t, &out)
	require.Len(t, logins, 1)
	assert.Empty(t, logins[0].Password)
	assert.Len(t, out.fails, 1)
}

func TestYandexLoginAAD_NoMasterPassword(t *testing.T) {
	got := yandexLoginAAD("https://example.com/", "user", "alice", "pass", "https://example.com/", nil)

	hasher := sha1.New()
	hasher.Write([]byte("https://example.com/\x00user\x00alice\x00pass\x00https://example.com/"))
	want := hasher.Sum(nil)

	assert.Equal(t, want, got)
	assert.Len(t, got, sha1.Size)
}

func TestYandexLoginAAD_WithMasterPassword(t *testing.T) {
	keyID := []byte("abc123")
	got := yandexLoginAAD("u", "e1", "v1", "e2", "r", keyID)

	require.Len(t, got, sha1.Size+len(keyID))
	assert.Equal(t, keyID, got[sha1.Size:])
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
	err := extractYandexCreditCards(keysOf(masterkey.Set{masterkey.V10: masterKey}), path, &out)
	got := collected[record.CreditCard](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byGUID := map[string]int{}
	for i, card := range got {
		byGUID[card.GUID] = i
	}

	firstCard := got[byGUID["card-1"]]
	assert.Equal(t, "Alice Smith", firstCard.Name)
	assert.Equal(t, "Personal Visa", firstCard.NickName)
	assert.Equal(t, "2030", firstCard.ExpYear)
	assert.Equal(t, "06", firstCard.ExpMonth)
	assert.Equal(t, "4111111111111111", firstCard.Number)
	assert.Equal(t, "123", firstCard.CVC)
	assert.Equal(t, "main card", firstCard.Comment)

	secondCard := got[byGUID["card-2"]]
	assert.Equal(t, "5555555555554444", secondCard.Number)
	assert.Equal(t, "456", secondCard.CVC)
	assert.Empty(t, secondCard.Comment)
}

func TestExtractYandexCreditCards_WrongMasterKey(t *testing.T) {
	goodKey := bytes.Repeat([]byte{0x11}, 32)
	wrongKey := bytes.Repeat([]byte{0x99}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, 32)

	path := setupYandexCreditCardDB(t, goodKey, dataKey,
		yandexCreditCard{GUID: "g1", FullCardNumber: "4111"},
	)

	var out testCollector
	require.NoError(t, extractYandexCreditCards(keysOf(masterkey.Set{masterkey.V10: wrongKey}), path, &out))
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

func TestExtractYandexPasswords_EmptyTableFetchesNoKey(t *testing.T) {
	path := setupYandexPasswordDB(t, bytes.Repeat([]byte{0x11}, 32), bytes.Repeat([]byte{0x22}, 32), false)
	fetched := false
	keys := masterKeysFunc(func() masterkey.Set {
		fetched = true
		return nil
	})
	var out testCollector
	require.NoError(t, extractYandexPasswords(keys, path, &out))
	assert.False(t, fetched, "a table without rows must not trigger key retrieval")
}
