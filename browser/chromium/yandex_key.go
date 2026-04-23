package chromium

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

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
