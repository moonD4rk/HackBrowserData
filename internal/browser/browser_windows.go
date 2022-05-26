//go:build windows

package browser

import (
	"hack-browser-data/internal/item"
	"io/ioutil"
	"os"
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
			profilePath: getProfiles(chromeUserDataPath)[0],
			items:       item.DefaultChromium,
		},
		"edge": {
			name:        edgeName,
			profilePath: edgeProfilePath,
			items:       item.DefaultChromium,
		},
		"chromium": {
			name:        chromiumName,
			profilePath: getProfiles(chromiumUserDataPath)[0],
			items:       item.DefaultChromium,
		},
		"chrome-beta": {
			name:        chromeBetaName,
			profilePath: getProfiles(chromeBetaUserDataPath)[0],
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

func getProfiles(userDataPath string) []string {
	var res []string
	files, err := ioutil.ReadDir(userDataPath)

	if err != nil {

		res = append(res, userDataPath+"Default")
	}

	for _, f := range files {
		if f.IsDir() && f.Name() != "System Profile" {
			var thisPath = userDataPath + f.Name()
			_, err := os.Stat(thisPath + "/Login Data")
			if err == nil {
				res = append(res, userDataPath+f.Name())
			}
		}
	}
	return res
}

var (
	chromeUserDataPath     = homeDir + "/AppData/Local/Google/Chrome/User Data/"
	chromeBetaUserDataPath = homeDir + "/AppData/Local/Google/Chrome Beta/User Data/"
	chromiumUserDataPath   = homeDir + "/AppData/Local/Chromium/User Data/"
	edgeProfilePath        = homeDir + "/AppData/Local/Microsoft/Edge/User Data/Default/"
	braveProfilePath       = homeDir + "/AppData/Local/BraveSoftware/Brave-Browser/User Data/Default/"
	speed360ProfilePath    = homeDir + "/AppData/Local/360chrome/Chrome/User Data/Default/"
	qqBrowserProfilePath   = homeDir + "/AppData/Local/Tencent/QQBrowser/User Data/Default/"
	operaProfilePath       = homeDir + "/AppData/Roaming/Opera Software/Opera Stable/Default/"
	operaGXProfilePath     = homeDir + "/AppData/Roaming/Opera Software/Opera GX Stable/"
	vivaldiProfilePath     = homeDir + "/AppData/Local/Vivaldi/User Data/Default/"
	coccocProfilePath      = homeDir + "/AppData/Local/CocCoc/Browser/User Data/Default/"
	yandexProfilePath      = homeDir + "/AppData/Local/Yandex/YandexBrowser/User Data/Default/"

	firefoxProfilePath = homeDir + "/AppData/Roaming/Mozilla/Firefox/Profiles/"
)
