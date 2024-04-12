//go:build linux

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
		"vivaldi": {
			name:        vivaldiName,
			storage:     vivaldiStorageName,
			profilePath: vivaldiProfilePath,
			dataTypes:   types.DefaultChromiumTypes,
		},
		"brave": {
			name:        braveName,
			profilePath: braveProfilePath,
			storage:     braveStorageName,
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
	firefoxProfilePath    = homeDir + "/.mozilla/firefox/"
	chromeProfilePath     = homeDir + "/.config/google-chrome/Default/"
	chromiumProfilePath   = homeDir + "/.config/chromium/Default/"
	edgeProfilePath       = homeDir + "/.config/microsoft-edge/Default/"
	braveProfilePath      = homeDir + "/.config/BraveSoftware/Brave-Browser/Default/"
	chromeBetaProfilePath = homeDir + "/.config/google-chrome-beta/Default/"
	operaProfilePath      = homeDir + "/.config/opera/Default/"
	vivaldiProfilePath    = homeDir + "/.config/vivaldi/Default/"
)

const (
	chromeStorageName     = "Chrome Safe Storage"
	chromiumStorageName   = "Chromium Safe Storage"
	edgeStorageName       = "Chromium Safe Storage"
	braveStorageName      = "Brave Safe Storage"
	chromeBetaStorageName = "Chrome Safe Storage"
	operaStorageName      = "Chromium Safe Storage"
	vivaldiStorageName    = "Chrome Safe Storage"
)
