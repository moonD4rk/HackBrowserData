package chromium

import (
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/types"
)

// yandexSourceOverrides contains only the entries that differ from chromiumSources.
var yandexSourceOverrides = map[types.Category][]sourcePath{
	types.Password:   {file("Ya Passman Data")},
	types.CreditCard: {file("Ya Credit Cards")},
}

// yandexSources returns chromiumSources with Yandex-specific overrides applied.
func yandexSources() map[types.Category][]sourcePath {
	sources := make(map[types.Category][]sourcePath, len(chromiumSources))
	for k, v := range chromiumSources {
		sources[k] = v
	}
	for k, v := range yandexSourceOverrides {
		sources[k] = v
	}
	return sources
}

// yandexExtractors overrides Password and CreditCard extraction for Yandex, which wraps its data-encryption key inside
// meta.local_encryptor_data, binds per-row AAD to GCM, and stores cards as JSON blobs in a records table.
var yandexExtractors = map[types.Category]categoryExtractor{
	types.Password:   passwordExtractor{fn: extractYandexPasswords},
	types.CreditCard: creditCardExtractor{fn: extractYandexCreditCards},
}

// yandexLoginAAD is SHA1(origin_url \x00 username_element \x00 username_value \x00 password_element \x00 signon_realm),
// with keyID appended when the profile has a master password (v1 always passes nil).
func yandexLoginAAD(originURL, usernameElem, usernameVal, passwordElem, signonRealm string, keyID []byte) []byte {
	h := sha1.New()
	h.Write([]byte(originURL))
	h.Write([]byte{0})
	h.Write([]byte(usernameElem))
	h.Write([]byte{0})
	h.Write([]byte(usernameVal))
	h.Write([]byte{0})
	h.Write([]byte(passwordElem))
	h.Write([]byte{0})
	h.Write([]byte(signonRealm))
	sum := h.Sum(nil)
	if len(keyID) == 0 {
		return sum
	}
	out := make([]byte, 0, len(sum)+len(keyID))
	out = append(out, sum...)
	out = append(out, keyID...)
	return out
}

// yandexCardAAD is the raw guid bytes (+ keyID if the profile has a master password).
func yandexCardAAD(guid string, keyID []byte) []byte {
	if len(keyID) == 0 {
		return []byte(guid)
	}
	out := make([]byte, 0, len(guid)+len(keyID))
	out = append(out, guid...)
	out = append(out, keyID...)
	return out
}

// errYandexMasterPasswordSet: caller warns + skips; RSA-OAEP unseal is deferred (RFC-012 §6).
var errYandexMasterPasswordSet = errors.New("yandex: profile protected by master password, skipping")

// loadYandexDataKey honors the master-password gate and returns the per-DB data key. See RFC-012 §4.2.
func loadYandexDataKey(dbPath string, masterKey []byte) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, fmt.Errorf("yandex: master key not available")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("yandex db file: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if hasMasterPassword(db) {
		return nil, errYandexMasterPasswordSet
	}

	var blob []byte
	if err := db.QueryRow("SELECT value FROM meta WHERE key = 'local_encryptor_data'").Scan(&blob); err != nil {
		return nil, fmt.Errorf("read local_encryptor_data: %w", err)
	}

	dataKey, err := crypto.DecryptYandexIntermediateKey(masterKey, blob)
	if err != nil {
		return nil, fmt.Errorf("derive yandex data key: %w", err)
	}
	return dataKey, nil
}

// hasMasterPassword: missing table (Ya Credit Cards) or empty sealed_key both mean false.
func hasMasterPassword(db *sql.DB) bool {
	var sealed sql.NullString
	if err := db.QueryRow("SELECT sealed_key FROM active_keys").Scan(&sealed); err != nil {
		return false
	}
	return sealed.Valid && strings.TrimSpace(sealed.String) != ""
}
