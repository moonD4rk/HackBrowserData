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
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
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

func extractPasswords(masterKey []byte, path string) ([]types.LoginEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var logins []types.LoginEntry
	for _, v := range gjson.GetBytes(data, "logins").Array() {
		user, err := decryptPBE(v.Get("encryptedUsername").String(), masterKey)
		if err != nil {
			log.Debugf("decrypt username: %v", err)
			continue
		}
		pwd, err := decryptPBE(v.Get("encryptedPassword").String(), masterKey)
		if err != nil {
			log.Debugf("decrypt password: %v", err)
			continue
		}

		url := v.Get("formSubmitURL").String()
		if url == "" {
			url = v.Get("hostname").String()
		}

		logins = append(logins, types.LoginEntry{
			URL:       url,
			Username:  string(user),
			Password:  string(pwd),
			CreatedAt: typeutil.TimeStamp(v.Get("timeCreated").Int() / 1000),
		})
	}

	sort.Slice(logins, func(i, j int) bool {
		return logins[i].CreatedAt.After(logins[j].CreatedAt)
	})
	return logins, nil
}
