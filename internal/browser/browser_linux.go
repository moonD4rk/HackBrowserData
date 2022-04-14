//go:build linux

package browser

import (
	"hack-browser-data/internal/item"
)

var (
	chromiumList = map[string]struct {
		name        string
		storage     string
		profilePath string
		items       []item.Item
	}{
		"chrome": {
			name:        chromeName,
			storage:     chromeStorageName,
			profilePath: chromeProfilePath,
			items:       item.DefaultChromium,
		},
		"edge": {
			name:        edgeName,
			storage:     edgeStorageName,
			profilePath: edgeProfilePath,
			items:       item.DefaultChromium,
		},
		"chromium": {
			name:        chromiumName,
			storage:     chromiumStorageName,
			profilePath: chromiumProfilePath,
			items:       item.DefaultChromium,
		},
		"chrome-beta": {
			name:        chromeBetaName,
			storage:     chromeBetaStorageName,
			profilePath: chromeBetaProfilePath,
			items:       item.DefaultChromium,
		},
		"opera": {
			name:        operaName,
			profilePath: operaProfilePath,
			storage:     operaStorageName,
			items:       item.DefaultChromium,
		},
		"vivaldi": {
			name:        vivaldiName,
			storage:     vivaldiStorageName,
			profilePath: vivaldiProfilePath,
			items:       item.DefaultChromium,
		},
		"brave": {
			name:        braveName,
			profilePath: braveProfilePath,
			storage:     braveStorageName,
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
	firefoxProfilePath    = homeDir + "/.mozilla/firefox/"
	chromeProfilePath     = homeDir + "/.config/google-chrome/Default/"
	chromiumProfilePath   = homeDir + "/.config/chromium/Default/"
	edgeProfilePath       = homeDir + "/.config/microsoft-edge*/Default/"
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
