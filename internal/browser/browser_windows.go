package browser

import (
	item2 "hack-browser-data/internal/item"
)

var (
	chromiumList = map[string]struct {
		browserInfo *browserInfo
		items       []item2.Item
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
		items       []item2.Item
	}{
		"firefox": {
			browserInfo: firefoxInfo,
			items:       defaultFirefoxItems,
		},
	}
)

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
