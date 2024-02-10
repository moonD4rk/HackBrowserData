//go:build darwin

package browser

import (
	"github.com/moond4rk/hackbrowserdata/types"
)

var (
	chromiumList = map[string]struct {
		name        string
		storage     string
		profilePath string
		items       []types.DataType
	}{
		"chrome": {
			name:        chromeName,
			storage:     chromeStorageName,
			profilePath: chromeProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"edge": {
			name:        edgeName,
			storage:     edgeStorageName,
			profilePath: edgeProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"chromium": {
			name:        chromiumName,
			storage:     chromiumStorageName,
			profilePath: chromiumProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"chrome-beta": {
			name:        chromeBetaName,
			storage:     chromeBetaStorageName,
			profilePath: chromeBetaProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"opera": {
			name:        operaName,
			profilePath: operaProfilePath,
			storage:     operaStorageName,
			items:       types.DefaultChromiumTypes,
		},
		"opera-gx": {
			name:        operaGXName,
			profilePath: operaGXProfilePath,
			storage:     operaStorageName,
			items:       types.DefaultChromiumTypes,
		},
		"vivaldi": {
			name:        vivaldiName,
			storage:     vivaldiStorageName,
			profilePath: vivaldiProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"coccoc": {
			name:        coccocName,
			storage:     coccocStorageName,
			profilePath: coccocProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"brave": {
			name:        braveName,
			profilePath: braveProfilePath,
			storage:     braveStorageName,
			items:       types.DefaultChromiumTypes,
		},
		"yandex": {
			name:        yandexName,
			storage:     yandexStorageName,
			profilePath: yandexProfilePath,
			items:       types.DefaultYandexTypes,
		},
		"arc": {
			name:        arcName,
			profilePath: arcProfilePath,
			storage:     arcStorageName,
			items:       types.DefaultChromiumTypes,
		},
	}
	firefoxList = map[string]struct {
		name        string
		storage     string
		profilePath string
		items       []types.DataType
	}{
		"firefox": {
			name:        firefoxName,
			profilePath: firefoxProfilePath,
			items:       types.DefaultFirefoxTypes,
		},
	}
)

var (
	chromeProfilePath     = homeDir + "/Library/Application Support/Google/Chrome/Default/"
	chromeBetaProfilePath = homeDir + "/Library/Application Support/Google/Chrome Beta/Default/"
	chromiumProfilePath   = homeDir + "/Library/Application Support/Chromium/Default/"
	edgeProfilePath       = homeDir + "/Library/Application Support/Microsoft Edge/Default/"
	braveProfilePath      = homeDir + "/Library/Application Support/BraveSoftware/Brave-Browser/Default/"
	operaProfilePath      = homeDir + "/Library/Application Support/com.operasoftware.Opera/Default/"
	operaGXProfilePath    = homeDir + "/Library/Application Support/com.operasoftware.OperaGX/Default/"
	vivaldiProfilePath    = homeDir + "/Library/Application Support/Vivaldi/Default/"
	coccocProfilePath     = homeDir + "/Library/Application Support/Coccoc/Default/"
	yandexProfilePath     = homeDir + "/Library/Application Support/Yandex/YandexBrowser/Default/"
	arcProfilePath        = homeDir + "/Library/Application Support/Arc/User Data/Default"

	firefoxProfilePath = homeDir + "/Library/Application Support/Firefox/Profiles/"
)

const (
	chromeStorageName     = "Chrome"
	chromeBetaStorageName = "Chrome"
	chromiumStorageName   = "Chromium"
	edgeStorageName       = "Microsoft Edge"
	braveStorageName      = "Brave"
	operaStorageName      = "Opera"
	vivaldiStorageName    = "Vivaldi"
	coccocStorageName     = "CocCoc"
	yandexStorageName     = "Yandex"
	arcStorageName        = "Arc"
)
