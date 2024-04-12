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
		dataTypes   []types.DataType
	}{
		"chrome": {
			name:        chromeName,
			storage:     chromeStorageName,
			profilePath: chromeProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"edge": {
			name:        edgeName,
			storage:     edgeStorageName,
			profilePath: edgeProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"chromium": {
			name:        chromiumName,
			storage:     chromiumStorageName,
			profilePath: chromiumProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"chrome-beta": {
			name:        chromeBetaName,
			storage:     chromeBetaStorageName,
			profilePath: chromeBetaProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"opera": {
			name:        operaName,
			profilePath: operaProfilePath,
			storage:     operaStorageName,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"opera-gx": {
			name:        operaGXName,
			profilePath: operaGXProfilePath,
			storage:     operaStorageName,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"vivaldi": {
			name:        vivaldiName,
			storage:     vivaldiStorageName,
			profilePath: vivaldiProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"coccoc": {
			name:        coccocName,
			storage:     coccocStorageName,
			profilePath: coccocProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"brave": {
			name:        braveName,
			profilePath: braveProfilePath,
			storage:     braveStorageName,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"yandex": {
			name:        yandexName,
			storage:     yandexStorageName,
			profilePath: yandexProfilePath,
			dataTypes:   types.DefaultYandexTypes,
		},
		"arc": {
			name:        arcName,
			profilePath: arcProfilePath,
			storage:     arcStorageName,
			dataTypes:   types.DefaultChromiumTypes,
		},
	}
	firefoxList = map[string]struct {
		name        string
		storage     string
		profilePath string
		dataTypes   []types.DataType
	}{
		"firefox": {
			name:        firefoxName,
			profilePath: firefoxProfilePath,
			dataTypes:   types.DefaultFirefoxTypes,
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
