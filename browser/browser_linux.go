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
		"vivaldi": {
			name:        vivaldiName,
			storage:     vivaldiStorageName,
			profilePath: vivaldiProfilePath,
			items:       types.DefaultChromiumTypes,
		},
		"brave": {
			name:        braveName,
			profilePath: braveProfilePath,
			storage:     braveStorageName,
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
