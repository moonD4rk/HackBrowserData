package browser

import (
	"encoding/base64"
	"errors"

	"github.com/tidwall/gjson"

	"hack-browser-data/internal/browser/consts"
	"hack-browser-data/internal/decrypter"
	"hack-browser-data/internal/utils"
)

var (
	chromiumList = map[string]struct {
		browserInfo *browserInfo
		items       []item
	}{
		"chrome": {
			browserInfo: chromeInfo,
			items:       defaultChromiumItems,
		},
		"edge": {
			browserInfo: edgeInfo,
			items:       defaultChromiumItems,
		},
		"yandex": {
			browserInfo: yandexInfo,
			items:       defaultYandexItems,
		},
	}
	firefoxList = map[string]struct {
		browserInfo *browserInfo
		items       []item
	}{
		"firefox": {
			browserInfo: firefoxInfo,
			items:       defaultFirefoxItems,
		},
	}
)

var (
	errDecodeMasterKeyFailed = errors.New("decode master key failed")
)

func (c *chromium) GetMasterKey() ([]byte, error) {
	keyFile, err := utils.ReadFile(consts.ChromiumKeyFilename)
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

var (
	chromeInfo = &browserInfo{
		name:        chromeName,
		profilePath: chromeProfilePath,
	}
	edgeInfo = &browserInfo{
		name:        edgeName,
		profilePath: edgeProfilePath,
	}
	yandexInfo = &browserInfo{
		name:        yandexName,
		profilePath: edgeProfilePath,
	}
	firefoxInfo = &browserInfo{
		name:        firefoxName,
		profilePath: firefoxProfilePath,
	}
)

const (
	chromeProfilePath     = "/AppData/Local/Google/Chrome/User Data/"
	chromeBetaProfilePath = "/AppData/Local/Google/Chrome Beta/User Data/"
	chromiumProfilePath   = "/AppData/Local/Chromium/User Data/"
	edgeProfilePath       = "/AppData/Local/Microsoft/Edge/User Data/"
	braveProfilePath      = "/AppData/Local/BraveSoftware/Brave-Browser/User Data/"
	speed360ProfilePath   = "/AppData/Local/360chrome/Chrome/User Data/"
	qqBrowserProfilePath  = "/AppData/Local/Tencent/QQBrowser/User Data/"
	operaProfilePath      = "/AppData/Roaming/Opera Software/Opera Stable/"
	operaGXProfilePath    = "/AppData/Roaming/Opera Software/Opera GX Stable/"
	vivaldiProfilePath    = "/AppData/Local/Vivaldi/User Data/Default/"
	coccocProfilePath     = "/AppData/Local/CocCoc/Browser/User Data/Default/"
	yandexProfilePath     = "/AppData/Local/Yandex/YandexBrowser/User Data/Default"

	firefoxProfilePath = "/AppData/Roaming/Mozilla/Firefox/Profiles"
)
