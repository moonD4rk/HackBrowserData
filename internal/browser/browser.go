package browser

import (
	"os"
	"strings"

	"hack-browser-data/internal/data"
)

type Browser interface {
	GetName() string

	GetMasterKey() ([]byte, error)

	GetBrowsingData() []data.BrowsingData

	CopyItemFileToLocal() error
}

var (
	// home dir path not for android and ios
	homeDir, _ = os.UserHomeDir()
)

func PickBrowser(name string) []Browser {
	var browsers []Browser
	clist := pickChromium(name)
	for _, b := range clist {
		if b != nil {
			browsers = append(browsers, b)
		}
	}
	flist := pickFirefox(name)
	for _, b := range flist {
		if b != nil {
			browsers = append(browsers, b)
		}
	}
	return browsers
}

func pickChromium(name string) []Browser {
	var browsers []Browser
	name = strings.ToLower(name)
	if name == "all" {
		for _, choice := range chromiumList {
			if b, err := newChromium(choice.browserInfo, choice.items); err == nil {
				browsers = append(browsers, b)
			} else {
				if strings.Contains(err.Error(), "profile path is not exist") {
					continue
				}
				panic(err)
			}
		}
		return browsers
	}
	if choice, ok := chromiumList[name]; ok {
		b, err := newChromium(choice.browserInfo, choice.items)
		if err != nil {
			panic(err)
		}
		browsers = append(browsers, b)
		return browsers
	}
	return nil
}

func pickFirefox(name string) []Browser {
	var browsers []Browser
	name = strings.ToLower(name)
	if name == "all" || name == "firefox" {
		for _, f := range firefoxList {
			multiFirefox, err := newMultiFirefox(f.browserInfo, f.items)
			if err != nil {
				panic(err)
			}
			for _, browser := range multiFirefox {
				browsers = append(browsers, browser)
			}
		}
		return browsers
	}
	return nil
}

func ListBrowser() []string {
	var l []string
	for c := range chromiumList {
		l = append(l, c)
	}
	for f := range firefoxList {
		l = append(l, f)
	}
	return l
}

func isFileExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

type browserInfo struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
}

const (
	chromeName         = "Chrome"
	chromeBetaName     = "Chrome Beta"
	chromiumName       = "Chromium"
	edgeName           = "Microsoft Edge"
	firefoxName        = "Firefox"
	firefoxBetaName    = "Firefox Beta"
	firefoxDevName     = "Firefox Dev"
	firefoxNightlyName = "Firefox Nightly"
	firefoxESRName     = "Firefox ESR"
	speed360Name       = "360speed"
	qqBrowserName      = "QQ"
	braveName          = "Brave"
	operaName          = "Opera"
	operaGXName        = "OperaGX"
	vivaldiName        = "Vivaldi"
	coccocName         = "CocCoc"
	yandexName         = "Yandex"
)
