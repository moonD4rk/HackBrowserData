//go:build windows

package browser

import (
	"github.com/moond4rk/hackbrowserdata/item"
)

var (
	firefoxList = make(map[string]struct {
		name        string
		storage     string
		profilePath string
		items       []item.Item
	})
	chromiumList = make(map[string]struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	})
)

func MakeUserFile(UserHomeDir string, username string) {
	chromeUserDataPath := UserHomeDir + "/AppData/Local/Google/Chrome/User Data/Default/"
	chromeBetaUserDataPath := UserHomeDir + "/AppData/Local/Google/Chrome Beta/User Data/Default/"
	chromiumUserDataPath := UserHomeDir + "/AppData/Local/Chromium/User Data/Default/"
	edgeProfilePath := UserHomeDir + "/AppData/Local/Microsoft/Edge/User Data/Default/"
	braveProfilePath := UserHomeDir + "/AppData/Local/BraveSoftware/Brave-Browser/User Data/Default/"
	speed360ProfilePath := UserHomeDir + "/AppData/Local/360chrome/Chrome/User Data/Default/"
	qqBrowserProfilePath := UserHomeDir + "/AppData/Local/Tencent/QQBrowser/User Data/Default/"
	operaProfilePath := UserHomeDir + "/AppData/Roaming/Opera Software/Opera Stable/"
	operaGXProfilePath := UserHomeDir + "/AppData/Roaming/Opera Software/Opera GX Stable/"
	vivaldiProfilePath := UserHomeDir + "/AppData/Local/Vivaldi/User Data/Default/"
	coccocProfilePath := UserHomeDir + "/AppData/Local/CocCoc/Browser/User Data/Default/"
	yandexProfilePath := UserHomeDir + "/AppData/Local/Yandex/YandexBrowser/User Data/Default/"
	dcBrowserProfilePath := UserHomeDir + "/AppData/Local/DCBrowser/User Data/Default/"
	sogouProfilePath := UserHomeDir + "/AppData/Roaming/SogouExplorer/Webkit/Default/"

	firefoxProfilePath := UserHomeDir + "/AppData/Roaming/Mozilla/Firefox/Profiles/"

	chromiumList[username+"chrome"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        chromeName,
		profilePath: chromeUserDataPath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"edge"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        edgeName,
		profilePath: edgeProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"chromium"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        chromiumName,
		profilePath: chromiumUserDataPath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"chrome-beta"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        chromeBetaName,
		profilePath: chromeBetaUserDataPath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"opera"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        operaName,
		profilePath: operaProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"opera-gx"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        operaGXName,
		profilePath: operaGXProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"vivaldi"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        vivaldiName,
		profilePath: vivaldiProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"coccoc"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        coccocName,
		profilePath: coccocProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"brave"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        braveName,
		profilePath: braveProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"yandex"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        yandexName,
		profilePath: yandexProfilePath,
		items:       item.DefaultYandex,
	}
	chromiumList[username+"360"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        speed360Name,
		profilePath: speed360ProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"qq"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        qqBrowserName,
		profilePath: qqBrowserProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"dc"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        dcBrowserName,
		profilePath: dcBrowserProfilePath,
		items:       item.DefaultChromium,
	}
	chromiumList[username+"sogou"] = struct {
		name        string
		profilePath string
		storage     string
		items       []item.Item
	}{
		name:        sogouName,
		profilePath: sogouProfilePath,
		items:       item.DefaultChromium,
	}
	firefoxList[username+"firefox"] = struct {
		name        string
		storage     string
		profilePath string
		items       []item.Item
	}{
		name:        firefoxName,
		profilePath: firefoxProfilePath,
		items:       item.DefaultFirefox,
	}
}
