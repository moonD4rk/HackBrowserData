//go:build windows

package browser

import (
	"hack-browser-data/internal/item"
)

var (
	chromiumList = map[string]struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		"chrome": {
			name:        chromeName,
			profilePath: chromeUserDataPath,
			items:       item.DefaultChromium,
		},
		"edge": {
			name:        edgeName,
			profilePath: edgeProfilePath,
			items:       item.DefaultChromium,
		},
		"chromium": {
			name:        chromiumName,
			profilePath: chromiumUserDataPath,
			items:       item.DefaultChromium,
		},
		"chrome-beta": {
			name:        chromeBetaName,
			profilePath: chromeBetaUserDataPath,
			items:       item.DefaultChromium,
		},
		"opera": {
			name:        operaName,
			profilePath: operaProfilePath,
			items:       item.DefaultChromium,
		},
		"opera-gx": {
			name:        operaGXName,
			profilePath: operaGXProfilePath,
			items:       item.DefaultChromium,
		},
		"vivaldi": {
			name:        vivaldiName,
			profilePath: vivaldiProfilePath,
			items:       item.DefaultChromium,
		},
		"coccoc": {
			name:        coccocName,
			profilePath: coccocProfilePath,
			items:       item.DefaultChromium,
		},
		"brave": {
			name:        braveName,
			profilePath: braveProfilePath,
			items:       item.DefaultChromium,
		},
		"yandex": {
			name:        yandexName,
			profilePath: yandexProfilePath,
			items:       item.DefaultYandex,
		},
		"360": {
			name:        speed360Name,
			profilePath: speed360ProfilePath,
			items:       item.DefaultChromium,
		},
		"qq": {
			name:        qqBrowserName,
			profilePath: qqBrowserProfilePath,
			items:       item.DefaultChromium,
		},
	}
	firefoxList = map[string]struct {
		name        string
		storage     string
		profilePath string
		items       []item.Item
	}{
		"firefox": {
			name:        firefoxName,
			profilePath: firefoxProfilePath,
			items:       item.DefaultFirefox,
		},
	}
)

var (
	chromeUserDataPath     = homeDir + "/AppData/Local/Google/Chrome/User Data/Default/"
	chromeBetaUserDataPath = homeDir + "/AppData/Local/Google/Chrome Beta/User Data/Default/"
	chromiumUserDataPath   = homeDir + "/AppData/Local/Chromium/User Data/Default/"
	edgeProfilePath        = homeDir + "/AppData/Local/Microsoft/Edge/User Data/Default/"
	braveProfilePath       = homeDir + "/AppData/Local/BraveSoftware/Brave-Browser/User Data/Default/"
	speed360ProfilePath    = homeDir + "/AppData/Local/360chrome/Chrome/User Data/Default/"
	qqBrowserProfilePath   = homeDir + "/AppData/Local/Tencent/QQBrowser/User Data/Default/"
	operaProfilePath       = homeDir + "/AppData/Roaming/Opera Software/Opera Stable/"
	operaGXProfilePath     = homeDir + "/AppData/Roaming/Opera Software/Opera GX Stable/"
	vivaldiProfilePath     = homeDir + "/AppData/Local/Vivaldi/User Data/Default/"
	coccocProfilePath      = homeDir + "/AppData/Local/CocCoc/Browser/User Data/Default/"
	yandexProfilePath      = homeDir + "/AppData/Local/Yandex/YandexBrowser/User Data/Default/"

	firefoxProfilePath = homeDir + "/AppData/Roaming/Mozilla/Firefox/Profiles/"
)
