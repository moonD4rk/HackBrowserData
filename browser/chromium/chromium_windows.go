//go:build windows

package chromium

import (
	"encoding/base64"
	"errors"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/item"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

var errDecodeMasterKeyFailed = errors.New("decode master key failed")

func (c *Chromium) GetMasterKey() ([]byte, error) {
	b, err := fileutil.ReadFile(item.TempChromiumKey)
	if err != nil {
		return nil, err
	}
	defer os.Remove(item.TempChromiumKey)

	encryptedKey := gjson.Get(b, "os_crypt.encrypted_key")
	if !encryptedKey.Exists() {
		return nil, nil
	}

	key, err := base64.StdEncoding.DecodeString(encryptedKey.String())
	if err != nil {
		return nil, errDecodeMasterKeyFailed
	}
	c.masterKey, err = crypto.DPAPI(key[5:])
	if err != nil {
		log.Errorf("%s failed to decrypt master key, maybe this profile was created on a different OS installation", c.name)
		return nil, err
	}
	log.Infof("%s initialized master key success", c.name)
	return c.masterKey, nil
}
