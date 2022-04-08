package chromium

import (
	"encoding/base64"
	"errors"

	"github.com/smallstep/cli/utils"
	"github.com/tidwall/gjson"
)

var (
	errDecodeMasterKeyFailed = errors.New("decode master key failed")
)

func (c *chromium) GetMasterKey() ([]byte, error) {
	keyFile, err := utils.ReadFile(item.TempChromiumKey)
	if err != nil {
		return nil, err
	}
	encryptedKey := gjson.Get(keyFile, "os_crypt.encrypted_key")
	if encryptedKey.Exists() {
		pureKey, err := base64.StdEncoding.DecodeString(encryptedKey.String())
		if err != nil {
			return nil, errDecodeMasterKeyFailed
		}
		c.browserInfo.masterKey, err = decrypter.DPApi(pureKey[5:])
		return c.browserInfo.masterKey, err
	}
	return nil, nil
}
