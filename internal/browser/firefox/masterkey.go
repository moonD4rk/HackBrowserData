package firefox

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

// retrieveMasterKey derives the key candidates in key4.db and returns the one that opens the logins in loginsPath.
func retrieveMasterKey(key4Path, loginsPath string) ([]byte, error) {
	key4, err := readKey4DB(key4Path)
	if err != nil {
		return nil, err
	}

	keys, err := key4.deriveKeys()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("no valid master key candidates in key4.db")
	}

	if key := validateKeyWithLogins(keys, loginsPath); key != nil {
		return key, nil
	}

	return nil, fmt.Errorf("derived %d key(s) but none could decrypt logins", len(keys))
}

// key4DB mirrors key4.db; see https://searchfox.org/mozilla-central/source/security/nss/lib/softoken/ for its layout.
type key4DB struct {
	globalSalt    []byte       // metaData.item1: salt used as PBE decryption input
	passwordCheck []byte       // metaData.item2: encrypted marker to verify DB integrity
	privateKeys   []privateKey // nssPrivate rows: encrypted master key candidates
}

type privateKey struct {
	encrypted []byte // a11: PBE-encrypted master key blob
	typeTag   []byte // a102: key type identifier (must match nssKeyTypeTag)
}

// nssKeyTypeTag is the a102 value of the nssPrivate rows that hold keys; see softoken/pkcs11i.h in NSS.
var nssKeyTypeTag = []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

func readKey4DB(path string) (*key4DB, error) {
	database, err := sqlitex.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open key4.db: %w", err)
	}
	defer database.Close()

	var record key4DB

	const metaQuery = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	if err := database.QueryRow(metaQuery).Scan(&record.globalSalt, &record.passwordCheck); err != nil {
		return nil, fmt.Errorf("query metaData: %w", err)
	}

	const nssQuery = `SELECT a11, a102 FROM nssPrivate`
	rows, err := database.Query(nssQuery)
	if err != nil {
		return nil, fmt.Errorf("query nssPrivate: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry privateKey
		if err := rows.Scan(&entry.encrypted, &entry.typeTag); err != nil {
			return nil, fmt.Errorf("scan nssPrivate row: %w", err)
		}
		record.privateKeys = append(record.privateKeys, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nssPrivate: %w", err)
	}
	if len(record.privateKeys) == 0 {
		return nil, errors.New("nssPrivate table is empty")
	}

	return &record, nil
}

func (k *key4DB) deriveKeys() ([][]byte, error) {
	if err := k.verifyPasswordCheck(); err != nil {
		return nil, err
	}

	var keys [][]byte
	for _, entry := range k.privateKeys {
		if !bytes.Equal(entry.typeTag, nssKeyTypeTag) {
			continue
		}
		key, err := k.decryptPrivateKey(entry)
		if err != nil {
			log.Debugf("decrypt nss private key: %v", err)
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

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

func (k *key4DB) decryptPrivateKey(entry privateKey) ([]byte, error) {
	pbe, err := crypto.NewASN1PBE(entry.encrypted)
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
	// Firefox 144+ uses AES-256-CBC, so the whole derived key is kept rather than 3DES's 24 bytes.
	return derivedKey, nil
}

type encryptedLogin struct {
	username []byte
	password []byte
}

// validateKeyWithLogins returns nil when no key decrypts a login.
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

// sampleEncryptedLogins takes up to 5 logins, enough to tell which key opens them.
func sampleEncryptedLogins(raw []byte) []encryptedLogin {
	logins := gjson.GetBytes(raw, "logins").Array()
	var samples []encryptedLogin
	for _, value := range logins {
		userRaw, err := base64.StdEncoding.DecodeString(value.Get("encryptedUsername").String())
		if err != nil {
			continue
		}
		pwdRaw, err := base64.StdEncoding.DecodeString(value.Get("encryptedPassword").String())
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
