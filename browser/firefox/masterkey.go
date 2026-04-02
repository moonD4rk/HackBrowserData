package firefox

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

// loginCipherPair holds base64-decoded encrypted username/password from logins.json,
// used to validate a candidate master key.
type loginCipherPair struct {
	username []byte
	password []byte
}

// parseLoginCipherPairs extracts up to 5 encrypted username/password pairs
// from Firefox logins.json content for master key validation.
func parseLoginCipherPairs(raw []byte) ([]loginCipherPair, error) {
	arr := gjson.GetBytes(raw, "logins").Array()
	pairs := make([]loginCipherPair, 0, len(arr))
	for _, v := range arr {
		uEnc := v.Get("encryptedUsername").String()
		pEnc := v.Get("encryptedPassword").String()
		if uEnc == "" || pEnc == "" {
			continue
		}
		uRaw, err := base64.StdEncoding.DecodeString(uEnc)
		if err != nil {
			continue
		}
		pRaw, err := base64.StdEncoding.DecodeString(pEnc)
		if err != nil {
			continue
		}
		pairs = append(pairs, loginCipherPair{username: uRaw, password: pRaw})
		if len(pairs) >= 5 {
			break
		}
	}
	return pairs, nil
}

// canDecryptAnyLoginCipherPair checks if masterKey can decrypt at least one
// login entry. Used to validate the correct master key when multiple NSS
// private candidates exist.
func canDecryptAnyLoginCipherPair(masterKey []byte, pairs []loginCipherPair) bool {
	for _, pair := range pairs {
		uPBE, err := crypto.NewASN1PBE(pair.username)
		if err != nil {
			continue
		}
		if _, err := uPBE.Decrypt(masterKey); err != nil {
			continue
		}

		pPBE, err := crypto.NewASN1PBE(pair.password)
		if err != nil {
			continue
		}
		if _, err := pPBE.Decrypt(masterKey); err == nil {
			return true
		}
	}
	return false
}

// nssPrivateCandidate holds a row from the nssPrivate table in key4.db.
type nssPrivateCandidate struct {
	a11  []byte
	a102 []byte
}

// queryMetaData reads the password-check metadata from key4.db.
func queryMetaData(db *sql.DB) ([]byte, []byte, error) {
	const query = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	var metaItem1, metaItem2 []byte
	if err := db.QueryRow(query).Scan(&metaItem1, &metaItem2); err != nil {
		return nil, nil, err
	}
	return metaItem1, metaItem2, nil
}

// queryNssPrivateCandidates reads all NSS private key candidates from key4.db.
func queryNssPrivateCandidates(db *sql.DB) ([]nssPrivateCandidate, error) {
	const query = `SELECT a11, a102 FROM nssPrivate`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []nssPrivateCandidate
	for rows.Next() {
		var c nssPrivateCandidate
		if err := rows.Scan(&c.a11, &c.a102); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, errors.New("nssPrivate is empty")
	}
	return candidates, nil
}

// queryNssPrivate returns the first NSS private key candidate.
// Kept for backward compatibility in tests.
func queryNssPrivate(db *sql.DB) ([]byte, []byte, error) {
	candidates, err := queryNssPrivateCandidates(db)
	if err != nil {
		return nil, nil, err
	}
	return candidates[0].a11, candidates[0].a102, nil
}

// processMasterKey derives the Firefox master key from key4.db data.
// It decrypts metaItem2 with ASN1PBE, verifies the "password-check" flag,
// validates nssA102, then decrypts nssA11 to obtain the final key.
func processMasterKey(metaItem1, metaItem2, nssA11, nssA102 []byte) ([]byte, error) {
	metaPBE, err := crypto.NewASN1PBE(metaItem2)
	if err != nil {
		return nil, fmt.Errorf("error creating ASN1PBE from metaItem2: %w", err)
	}

	flag, err := metaPBE.Decrypt(metaItem1)
	if err != nil {
		return nil, fmt.Errorf("error decrypting master key: %w", err)
	}
	const passwordCheck = "password-check"

	if !bytes.Contains(flag, []byte(passwordCheck)) {
		return nil, errors.New("flag verification failed: password-check not found")
	}

	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if !bytes.Equal(nssA102, keyLin) {
		return nil, errors.New("master key verification failed: nssA102 not equal to expected value")
	}

	nssA11PBE, err := crypto.NewASN1PBE(nssA11)
	if err != nil {
		return nil, fmt.Errorf("error creating ASN1PBE from nssA11: %w", err)
	}

	derivedKey, err := nssA11PBE.Decrypt(metaItem1)
	if err != nil {
		return nil, fmt.Errorf("error decrypting final key: %w", err)
	}
	if len(derivedKey) < 24 {
		return nil, errors.New("length of final key is less than 24 bytes")
	}
	// Historically, the derived PBE key was truncated to 24 bytes for 3DES usage.
	// Starting from Firefox 144+, NSS switches to AES-256-CBC without changing
	// the underlying key derivation logic. The full derived key must be preserved
	// to support modern cipher suites.
	return derivedKey, nil
}
