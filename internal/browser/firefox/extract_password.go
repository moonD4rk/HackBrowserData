package firefox

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/crypto"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

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

func extractPasswords(masterKey func() []byte, path string, out browser.Collector) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var logins []record.Login
	for _, v := range gjson.GetBytes(data, "logins").Array() {
		url := v.Get("formSubmitURL").String()
		if url == "" {
			url = v.Get("hostname").String()
		}

		key := masterKey()
		user, err := decryptPBE(v.Get("encryptedUsername").String(), key)
		if err != nil {
			out.Fail(fmt.Errorf("decrypt username for %s: %w", url, err))
		}
		pwd, err := decryptPBE(v.Get("encryptedPassword").String(), key)
		if err != nil {
			out.Fail(fmt.Errorf("decrypt password for %s: %w", url, err))
		}

		logins = append(logins, record.Login{
			URL:       url,
			Username:  string(user),
			Password:  string(pwd),
			CreatedAt: timeconv.UnixMilli(v.Get("timeCreated").Int()),
			Store:     record.StoreLocal,
		})
	}
	emit(out, logins)
	return nil
}
