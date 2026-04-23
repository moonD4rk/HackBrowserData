package firefox

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

func countPasswords(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return len(gjson.GetBytes(data, "logins").Array()), nil
}

// decryptPBE combines base64 decode + ASN1 PBE parse + decrypt into one call.
func decryptPBE(encoded string, masterKey []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	pbe, err := crypto.NewASN1PBE(raw)
	if err != nil {
		return nil, fmt.Errorf("parse asn1 pbe: %w", err)
	}
	plaintext, err := pbe.Decrypt(masterKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

func extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var logins []types.LoginEntry
	var decryptFails int
	var lastErr error
	for _, v := range gjson.GetBytes(data, "logins").Array() {
		url := v.Get("formSubmitURL").String()
		if url == "" {
			url = v.Get("hostname").String()
		}

		user, err := decryptPBE(v.Get("encryptedUsername").String(), masterKey)
		if err != nil {
			decryptFails++
			lastErr = err
		}
		pwd, err := decryptPBE(v.Get("encryptedPassword").String(), masterKey)
		if err != nil {
			decryptFails++
			lastErr = err
		}

		logins = append(logins, types.LoginEntry{
			URL:       url,
			Username:  string(user),
			Password:  string(pwd),
			CreatedAt: firefoxMillis(v.Get("timeCreated").Int()),
		})
	}
	if decryptFails > 0 {
		log.Debugf("decrypt firefox login fields: %d failed: %v", decryptFails, lastErr)
	}

	sort.Slice(logins, func(i, j int) bool {
		return logins[i].CreatedAt.After(logins[j].CreatedAt)
	})
	return logins, nil
}
