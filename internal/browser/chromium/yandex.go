package chromium

import (
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
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
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("yandex db file: %w", err)
	}
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
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
