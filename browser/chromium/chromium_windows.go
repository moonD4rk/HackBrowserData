//go:build windows

package chromium

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/item"
	"github.com/moond4rk/hackbrowserdata/logger"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

var errDecodeMasterKeyFailed = errors.New("decode master key failed")

func (c *Chromium) GetMasterKey() ([]byte, error) {
	b, err := fileutil.ReadFile(item.ChromiumKey.TempFilename())
	if err != nil {
		return nil, err
	}
	defer os.Remove(item.ChromiumKey.TempFilename())

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
		logger.Errorf("%s failed to decrypt master key, maybe this profile was created on a different OS installation", c.name)
		return nil, err
	}
	slog.Info("get master key success", "browser", c.name)
	return c.masterKey, nil
}
