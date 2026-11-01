package chromium

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

// Ya Passman Data holds meta, active_keys, and logins; Ya Credit Cards holds meta and records.

const yandexLoginsSchema = `CREATE TABLE logins (
	origin_url VARCHAR NOT NULL,
	action_url VARCHAR,
	username_element VARCHAR,
	username_value VARCHAR,
	password_element VARCHAR,
	password_value BLOB,
	signon_realm VARCHAR NOT NULL,
	date_created INTEGER NOT NULL DEFAULT 0
)`

const yandexMetaSchema = `CREATE TABLE meta (
	key LONGVARCHAR NOT NULL UNIQUE PRIMARY KEY,
	value LONGVARCHAR
)`

const yandexActiveKeysSchema = `CREATE TABLE active_keys (
	key_id TEXT,
	sealed_key TEXT
)`

const yandexRecordsSchema = `CREATE TABLE records (
	guid TEXT PRIMARY KEY,
	public_data TEXT,
	private_data BLOB
)`

// A fixed nonce keeps failures reproducible by hand; Yandex itself draws a random one per row.
var yandexTestNonce = bytes.Repeat([]byte{0x77}, 12)

// Distinct from yandexTestNonce, so mixing the two up fails to decrypt instead of passing.
var yandexMasterKeyBlobNonce = bytes.Repeat([]byte{0xAB}, 12)

// Protobuf header: field 1 varint 1, field 2 length 32.
var yandexSignatureForFixtures = []byte{0x08, 0x01, 0x12, 0x20}

func yandexSealAESGCM(t *testing.T, key, nonce, plaintext, aad []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	return aead.Seal(nil, nonce, plaintext, aad)
}

// meta.local_encryptor_data: preamble, "v10", nonce, then signature and key sealed and padded to 68 bytes.
func buildYandexLocalEncryptorBlob(t *testing.T, masterKey, dataKey []byte) []byte {
	t.Helper()
	plaintext := append([]byte{}, yandexSignatureForFixtures...)
	plaintext = append(plaintext, dataKey...)
	// Pad to 68B (= 96 blob - 12 nonce - 16 tag) to match the on-disk shape.
	plaintext = append(plaintext, make([]byte, 68-len(plaintext))...)

	ciphertext := yandexSealAESGCM(t, masterKey, yandexMasterKeyBlobNonce, plaintext, nil)
	blob := []byte{0x12, 0x34, 0x56, 0x78} // arbitrary preamble
	blob = append(blob, "v10"...)
	blob = append(blob, yandexMasterKeyBlobNonce...)
	blob = append(blob, ciphertext...)
	return blob
}

type yandexPassword struct {
	OriginURL, UsernameElem, UsernameVal, PasswordElem, SignonRealm, Password string
	DateCreated                                                               int64
}

// hasMasterPassword adds a non-empty sealed_key row, as a master-password profile has.
func setupYandexPasswordDB(t *testing.T, masterKey, dataKey []byte, hasMasterPassword bool, rows ...yandexPassword) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Ya Passman Data")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	for _, schema := range []string{yandexLoginsSchema, yandexMetaSchema, yandexActiveKeysSchema} {
		_, err = db.Exec(schema)
		require.NoError(t, err)
	}

	blob := buildYandexLocalEncryptorBlob(t, masterKey, dataKey)
	_, err = db.Exec(`INSERT INTO meta (key, value) VALUES ('local_encryptor_data', ?)`, blob)
	require.NoError(t, err)

	if hasMasterPassword {
		_, err = db.Exec(`INSERT INTO active_keys (key_id, sealed_key) VALUES ('kid', 'sealed-opaque')`)
		require.NoError(t, err)
	}

	for _, r := range rows {
		aad := yandexLoginAAD(r.OriginURL, r.UsernameElem, r.UsernameVal, r.PasswordElem, r.SignonRealm, nil)
		ciphertext := yandexSealAESGCM(t, dataKey, yandexTestNonce, []byte(r.Password), aad)
		passwordBlob := append([]byte{}, yandexTestNonce...)
		passwordBlob = append(passwordBlob, ciphertext...)
		stmt := fmt.Sprintf(
			`INSERT INTO logins (origin_url, action_url, username_element, username_value,
				password_element, password_value, signon_realm, date_created)
			 VALUES ('%s', '', '%s', '%s', '%s', x'%s', '%s', %d)`,
			r.OriginURL, r.UsernameElem, r.UsernameVal, r.PasswordElem,
			hex.EncodeToString(passwordBlob), r.SignonRealm, r.DateCreated,
		)
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}
	return path
}

type yandexCreditCard struct {
	GUID                                     string
	CardHolder, CardTitle, ExpYear, ExpMonth string
	FullCardNumber, PinCode, SecretComment   string
}

func setupYandexCreditCardDB(t *testing.T, masterKey, dataKey []byte, rows ...yandexCreditCard) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Ya Credit Cards")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	for _, schema := range []string{yandexRecordsSchema, yandexMetaSchema} {
		_, err = db.Exec(schema)
		require.NoError(t, err)
	}

	blob := buildYandexLocalEncryptorBlob(t, masterKey, dataKey)
	_, err = db.Exec(`INSERT INTO meta (key, value) VALUES ('local_encryptor_data', ?)`, blob)
	require.NoError(t, err)

	for _, r := range rows {
		public := yandexPublicData{
			CardHolder:      r.CardHolder,
			CardTitle:       r.CardTitle,
			ExpireDateYear:  r.ExpYear,
			ExpireDateMonth: r.ExpMonth,
		}
		publicJSON, err := json.Marshal(public)
		require.NoError(t, err)

		private := yandexPrivateData{
			FullCardNumber: r.FullCardNumber,
			PinCode:        r.PinCode,
			SecretComment:  r.SecretComment,
		}
		privateJSON, err := json.Marshal(private)
		require.NoError(t, err)

		aad := yandexCardAAD(r.GUID, nil)
		ciphertext := yandexSealAESGCM(t, dataKey, yandexTestNonce, privateJSON, aad)
		privateBlob := append([]byte{}, yandexTestNonce...)
		privateBlob = append(privateBlob, ciphertext...)

		_, err = db.Exec(
			`INSERT INTO records (guid, public_data, private_data) VALUES (?, ?, ?)`,
			r.GUID, string(publicJSON), privateBlob,
		)
		require.NoError(t, err)
	}
	return path
}
