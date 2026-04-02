package firefox

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/tidwall/gjson"
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/log"
)

// key4DB holds the parsed contents of Firefox's key4.db NSS key storage.
//
// Firefox stores the master encryption key in key4.db using two SQLite tables:
//   - metaData: contains the global salt and an encrypted "password-check" marker
//   - nssPrivate: contains one or more encrypted master key candidates
//
// Reference: https://searchfox.org/mozilla-central/source/security/nss/lib/softoken/
type key4DB struct {
	globalSalt    []byte       // metaData.item1: salt used as PBE decryption input
	passwordCheck []byte       // metaData.item2: encrypted marker to verify DB integrity
	privateKeys   []privateKey // nssPrivate rows: encrypted master key candidates
}

// privateKey is a single encrypted master key entry from nssPrivate.
type privateKey struct {
	encrypted []byte // a11: PBE-encrypted master key blob
	typeTag   []byte // a102: key type identifier (must match nssKeyTypeTag)
}

// nssKeyTypeTag identifies valid master key entries in key4.db.
// Only nssPrivate rows where a102 matches this tag contain actual master keys;
// other rows may be certificates or other NSS objects.
// See: https://searchfox.org/mozilla-central/source/security/nss/lib/softoken/pkcs11i.h
var nssKeyTypeTag = []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

// readKey4DB opens key4.db and parses it into a structured key4DB.
func readKey4DB(path string) (*key4DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open key4.db: %w", err)
	}
	defer db.Close()

	var record key4DB

	// Read metaData table
	const metaQuery = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	if err := db.QueryRow(metaQuery).Scan(&record.globalSalt, &record.passwordCheck); err != nil {
		return nil, fmt.Errorf("query metaData: %w", err)
	}

	// Read nssPrivate table
	const nssQuery = `SELECT a11, a102 FROM nssPrivate`
	rows, err := db.Query(nssQuery)
	if err != nil {
		return nil, fmt.Errorf("query nssPrivate: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pk privateKey
		if err := rows.Scan(&pk.encrypted, &pk.typeTag); err != nil {
			return nil, fmt.Errorf("scan nssPrivate row: %w", err)
		}
		record.privateKeys = append(record.privateKeys, pk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nssPrivate: %w", err)
	}
	if len(record.privateKeys) == 0 {
		return nil, errors.New("nssPrivate table is empty")
	}

	return &record, nil
}

// deriveKeys verifies the database integrity via the password-check marker,
// then decrypts all valid master key candidates.
func (k *key4DB) deriveKeys() ([][]byte, error) {
	if err := k.verifyPasswordCheck(); err != nil {
		return nil, err
	}

	var keys [][]byte
	for _, pk := range k.privateKeys {
		if !bytes.Equal(pk.typeTag, nssKeyTypeTag) {
			continue
		}
		key, err := k.decryptPrivateKey(pk)
		if err != nil {
			log.Debugf("decrypt nss private key: %v", err)
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// verifyPasswordCheck decrypts the password-check marker from metaData
// to confirm the database is valid and accessible.
func (k *key4DB) verifyPasswordCheck() error {
	pbe, err := crypto.NewASN1PBE(k.passwordCheck)
	if err != nil {
		return fmt.Errorf("parse password check: %w", err)
	}
	plain, err := pbe.Decrypt(k.globalSalt)
	if err != nil {
		return fmt.Errorf("decrypt password check: %w", err)
	}
	if !bytes.Contains(plain, []byte("password-check")) {
		return errors.New("password check verification failed")
	}
	return nil
}

// decryptPrivateKey decrypts a single master key candidate using the global salt.
func (k *key4DB) decryptPrivateKey(pk privateKey) ([]byte, error) {
	pbe, err := crypto.NewASN1PBE(pk.encrypted)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	derivedKey, err := pbe.Decrypt(k.globalSalt)
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}
	if len(derivedKey) < 24 {
		return nil, fmt.Errorf("derived key too short: %d bytes (need >= 24)", len(derivedKey))
	}
	// Firefox 144+ uses AES-256-CBC instead of 3DES; the full derived key
	// must be preserved to support modern cipher suites.
	return derivedKey, nil
}

// encryptedLogin holds PBE-encrypted credentials from logins.json,
// used as test samples for master key validation.
type encryptedLogin struct {
	username []byte // PBE-encrypted username blob
	password []byte // PBE-encrypted password blob
}

// validateKeyWithLogins reads logins.json and returns the first key that
// can successfully decrypt an actual login entry. Returns nil if no key matches.
func validateKeyWithLogins(keys [][]byte, loginsPath string) []byte {
	raw, err := os.ReadFile(loginsPath)
	if err != nil {
		return nil
	}
	samples := sampleEncryptedLogins(raw)
	if len(samples) == 0 {
		return nil
	}
	for _, key := range keys {
		if tryDecryptLogins(key, samples) {
			return key
		}
	}
	return nil
}

// sampleEncryptedLogins extracts up to 5 encrypted login entries from
// logins.json as test samples for master key validation.
func sampleEncryptedLogins(raw []byte) []encryptedLogin {
	arr := gjson.GetBytes(raw, "logins").Array()
	var samples []encryptedLogin
	for _, v := range arr {
		userRaw, err := base64.StdEncoding.DecodeString(v.Get("encryptedUsername").String())
		if err != nil {
			continue
		}
		pwdRaw, err := base64.StdEncoding.DecodeString(v.Get("encryptedPassword").String())
		if err != nil {
			continue
		}
		samples = append(samples, encryptedLogin{username: userRaw, password: pwdRaw})
		if len(samples) >= 5 {
			break
		}
	}
	return samples
}

// tryDecryptLogins checks if masterKey can decrypt at least one encrypted
// login entry (both username and password).
func tryDecryptLogins(masterKey []byte, samples []encryptedLogin) bool {
	for _, login := range samples {
		userPBE, err := crypto.NewASN1PBE(login.username)
		if err != nil {
			continue
		}
		if _, err := userPBE.Decrypt(masterKey); err != nil {
			continue
		}
		pwdPBE, err := crypto.NewASN1PBE(login.password)
		if err != nil {
			continue
		}
		if _, err := pwdPBE.Decrypt(masterKey); err == nil {
			return true
		}
	}
	return false
}
