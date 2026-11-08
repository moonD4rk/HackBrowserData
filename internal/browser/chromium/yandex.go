package chromium

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

func yandexLoginAAD(originURL, usernameElement, usernameValue, passwordElement, signonRealm string, keyID []byte) []byte {
	hasher := sha1.New()
	hasher.Write([]byte(originURL))
	hasher.Write([]byte{0})
	hasher.Write([]byte(usernameElement))
	hasher.Write([]byte{0})
	hasher.Write([]byte(usernameValue))
	hasher.Write([]byte{0})
	hasher.Write([]byte(passwordElement))
	hasher.Write([]byte{0})
	hasher.Write([]byte(signonRealm))
	sum := hasher.Sum(nil)
	if len(keyID) == 0 {
		return sum
	}
	out := make([]byte, 0, len(sum)+len(keyID))
	out = append(out, sum...)
	out = append(out, keyID...)
	return out
}

func yandexCardAAD(guid string, keyID []byte) []byte {
	if len(keyID) == 0 {
		return []byte(guid)
	}
	out := make([]byte, 0, len(guid)+len(keyID))
	out = append(out, guid...)
	out = append(out, keyID...)
	return out
}

// errYandexMasterPasswordSet fails the extraction; unsealing a master-password profile is deferred.
var errYandexMasterPasswordSet = errors.New("yandex: profile protected by master password, skipping")

func loadYandexDataKey(dbPath string, masterKey []byte) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, fmt.Errorf("yandex: master key not available")
	}
	database, err := sqlitex.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("yandex db: %w", err)
	}
	defer database.Close()

	if hasMasterPassword(database) {
		return nil, errYandexMasterPasswordSet
	}

	var blob []byte
	if err := database.QueryRow("SELECT value FROM meta WHERE key = 'local_encryptor_data'").Scan(&blob); err != nil {
		return nil, fmt.Errorf("read local_encryptor_data: %w", err)
	}

	dataKey, err := decryptYandexIntermediateKey(masterKey, blob)
	if err != nil {
		return nil, fmt.Errorf("derive yandex data key: %w", err)
	}
	return dataKey, nil
}

// hasMasterPassword: missing table (Ya Credit Cards) or empty sealed_key both mean false.
func hasMasterPassword(database *sql.DB) bool {
	var sealed sql.NullString
	if err := database.QueryRow("SELECT sealed_key FROM active_keys").Scan(&sealed); err != nil {
		return false
	}
	return sealed.Valid && strings.TrimSpace(sealed.String) != ""
}

// yandexSignature is the protobuf wire-format header (field1 varint=1, field2 len=32) on every wrapped key.
var yandexSignature = []byte{0x08, 0x01, 0x12, 0x20}

var localEncryptorPrefix = []byte("v10")

const (
	yandexIntermediateKeyBlobLength = 96 // 12B nonce + 68B ciphertext + 16B GCM tag
	yandexDataKeyLength             = 32
)

var (
	errYandexMarkerNotFound = errors.New("yandex: v10 marker not found in local_encryptor_data")
	errYandexBlobShort      = errors.New("yandex: encrypted intermediate key truncated")
	errYandexBadSignature   = errors.New("yandex: invalid protobuf signature on decrypted key")
	errYandexKeyTooShort    = errors.New("yandex: decrypted intermediate key shorter than 32 bytes")
)

func decryptYandexIntermediateKey(masterKey, blob []byte) ([]byte, error) {
	index := bytes.Index(blob, localEncryptorPrefix)
	if index < 0 {
		return nil, errYandexMarkerNotFound
	}
	payload := blob[index+len(localEncryptorPrefix):]
	if len(payload) < yandexIntermediateKeyBlobLength {
		return nil, errYandexBlobShort
	}

	plaintext, err := crypto.AESGCMDecryptBlob(masterKey, payload[:yandexIntermediateKeyBlobLength], nil)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(plaintext, yandexSignature) {
		return nil, errYandexBadSignature
	}
	plaintext = plaintext[len(yandexSignature):]
	if len(plaintext) < yandexDataKeyLength {
		return nil, errYandexKeyTooShort
	}
	return plaintext[:yandexDataKeyLength], nil
}

const yandexLoginQuery = `SELECT origin_url, username_element, username_value,
		password_element, password_value, signon_realm, date_created FROM logins`

// extractYandexPasswords reads origin_url, which the per-row AAD covers, not action_url.
func extractYandexPasswords(masterKeys masterKeysFunc, path string, out browser.Collector) error {
	database, err := sqlitex.Open(path)
	if err != nil {
		return err
	}
	defer database.Close()

	rows, err := database.Query(yandexLoginQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	// The data key is loaded by the first row, so an empty table fetches no key.
	var (
		dataKey []byte
		keyErr  error
	)
	for rows.Next() {
		var originURL, usernameElement, usernameValue, passwordElement, signonRealm string
		var passwordValue []byte
		var created int64
		if err := rows.Scan(&originURL, &usernameElement, &usernameValue, &passwordElement, &passwordValue, &signonRealm, &created); err != nil {
			out.Fail(err)
			continue
		}
		entry := record.Login{
			URL:       originURL,
			Username:  usernameValue,
			CreatedAt: timeconv.Chromium(created),
			Store:     record.StoreLocal,
		}
		if dataKey == nil && keyErr == nil {
			dataKey, keyErr = loadYandexDataKey(path, masterKeys()[masterkey.V10])
		}
		if dataKey == nil {
			out.Fail(fmt.Errorf("yandex: password for %s: %w", originURL, keyErr))
			out.Add(&entry)
			continue
		}
		aad := yandexLoginAAD(originURL, usernameElement, usernameValue, passwordElement, signonRealm, nil)
		plaintext, err := crypto.AESGCMDecryptBlob(dataKey, passwordValue, aad)
		if err != nil {
			out.Fail(fmt.Errorf("yandex: decrypt password for %s: %w", originURL, err))
			out.Add(&entry)
			continue
		}
		entry.Password = string(plaintext)
		out.Add(&entry)
	}
	return rows.Err()
}

const yandexCreditCardQuery = `SELECT guid, public_data, private_data FROM records`

type yandexPublicData struct {
	CardHolder      string `json:"card_holder"`
	CardTitle       string `json:"card_title"`
	ExpireDateYear  string `json:"expire_date_year"`
	ExpireDateMonth string `json:"expire_date_month"`
}

type yandexPrivateData struct {
	FullCardNumber string `json:"full_card_number"`
	PinCode        string `json:"pin_code"`
	SecretComment  string `json:"secret_comment"`
}

// extractYandexCreditCards reads the records table (not Chromium's credit_cards). AAD = guid.
func extractYandexCreditCards(masterKeys masterKeysFunc, path string, out browser.Collector) error {
	database, err := sqlitex.Open(path)
	if err != nil {
		return err
	}
	defer database.Close()

	rows, err := database.Query(yandexCreditCardQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	// The data key is loaded by the first row, so an empty table fetches no key.
	var (
		dataKey []byte
		keyErr  error
	)
	for rows.Next() {
		var guid, publicData string
		var privateData []byte
		if err := rows.Scan(&guid, &publicData, &privateData); err != nil {
			out.Fail(err)
			continue
		}

		var public yandexPublicData
		if publicData != "" {
			if err := json.Unmarshal([]byte(publicData), &public); err != nil {
				out.Fail(fmt.Errorf("yandex: parse public_data for %s: %w", guid, err))
			}
		}
		entry := record.CreditCard{
			GUID:     guid,
			Name:     public.CardHolder,
			ExpMonth: public.ExpireDateMonth,
			ExpYear:  public.ExpireDateYear,
			NickName: public.CardTitle,
		}
		if dataKey == nil && keyErr == nil {
			dataKey, keyErr = loadYandexDataKey(path, masterKeys()[masterkey.V10])
		}
		if dataKey == nil {
			out.Fail(fmt.Errorf("yandex: card %s: %w", guid, keyErr))
			out.Add(&entry)
			continue
		}

		plaintext, err := crypto.AESGCMDecryptBlob(dataKey, privateData, yandexCardAAD(guid, nil))
		if err != nil {
			out.Fail(fmt.Errorf("yandex: decrypt card %s: %w", guid, err))
			out.Add(&entry)
			continue
		}

		var private yandexPrivateData
		if err := json.Unmarshal(plaintext, &private); err != nil {
			out.Fail(fmt.Errorf("yandex: parse private_data for %s: %w", guid, err))
			out.Add(&entry)
			continue
		}
		entry.Number = private.FullCardNumber
		entry.CVC = private.PinCode
		entry.Comment = private.SecretComment
		out.Add(&entry)
	}
	return rows.Err()
}
