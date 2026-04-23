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

// ---------------------------------------------------------------------------
// Yandex-specific SQLite schemas and test fixtures.
//
// Ya Passman Data:
//   - meta(key, value)              holds local_encryptor_data
//   - active_keys(key_id, sealed_key) non-empty sealed_key = master password set
//   - logins(...)                   same column set as Chromium, minus columns we
//                                   don't query, plus a signon_realm NOT NULL
//
// Ya Credit Cards:
//   - meta(key, value)              holds its own local_encryptor_data
//   - records(guid, public_data, private_data)
// ---------------------------------------------------------------------------

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

// yandexTestNonce is a fixed 12-byte nonce used across fixtures so test failures
// are easy to reproduce by hand. Real Yandex uses CSPRNG nonces per row.
var yandexTestNonce = bytes.Repeat([]byte{0x77}, 12)

// yandexMasterKeyBlobNonce is a fixed 12-byte nonce for sealing the intermediate
// data key inside meta.local_encryptor_data. Distinct from yandexTestNonce so a
// test mix-up surfaces as a decrypt error rather than a false pass.
var yandexMasterKeyBlobNonce = bytes.Repeat([]byte{0xAB}, 12)

// yandexSignatureForFixtures duplicates crypto.yandexSignature so tests can
// construct blobs without the crypto package exporting its internal constant.
// Protobuf header bytes: field1 varint=1, field2 len=32.
var yandexSignatureForFixtures = []byte{0x08, 0x01, 0x12, 0x20}

// yandexSealAESGCM seals plaintext under (key, nonce, aad) using AES-GCM.
func yandexSealAESGCM(t *testing.T, key, nonce, plaintext, aad []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	return aead.Seal(nil, nonce, plaintext, aad)
}

// buildYandexLocalEncryptorBlob produces the exact byte layout stored in
// meta.local_encryptor_data: [preamble]"v10"[12B nonce][68B plaintext + 16B GCM tag].
// The plaintext is signature (4B) + dataKey (32B) + zero padding to 68B.
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

// yandexPassword describes one row of test data for the logins table.
type yandexPassword struct {
	OriginURL, UsernameElem, UsernameVal, PasswordElem, SignonRealm, Password string
	DateCreated                                                               int64
}

// setupYandexPasswordDB creates a Ya Passman Data SQLite file with meta,
// active_keys, and logins populated. Each logins row is sealed under dataKey
// using the same per-row AAD derivation the production extractor expects.
// Set hasMasterPassword=true to simulate a profile protected by a master
// password (a non-empty sealed_key row).
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

// yandexCreditCard describes one row of test data for the records table.
type yandexCreditCard struct {
	GUID                                     string
	CardHolder, CardTitle, ExpYear, ExpMonth string
	FullCardNumber, PinCode, SecretComment   string
}

// setupYandexCreditCardDB creates a Ya Credit Cards SQLite file with meta and
// records populated. Each record's private_data is sealed under dataKey with
// AAD = guid bytes, matching the production extractor.
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
