package browser

import (
	"fmt"
	"os"
	"strings"

	"hack-browser-data/internal/browingdata"
	"hack-browser-data/internal/browser/chromium"
	"hack-browser-data/internal/browser/firefox"
)

type Browser interface {
	Name() string

	GetMasterKey() ([]byte, error)
	// GetBrowsingData returns the browsing data for the browser.
	GetBrowsingData() (*browingdata.Data, error)
}

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
		for _, c := range chromiumList {
			if b, err := chromium.New(c.name, c.storage, c.profilePath, c.items); err == nil {
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
	if c, ok := chromiumList[name]; ok {
		b, err := chromium.New(c.name, c.storage, c.profilePath, c.items)
		if err != nil {
			if strings.Contains(err.Error(), "profile path is not exist") {
				fmt.Println(err.Error())
			} else {
				panic(err)
			}
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
		for _, v := range firefoxList {
			multiFirefox, err := firefox.New(v.name, v.storage, v.profilePath, v.items)
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

var (
	// home dir path for all platforms
	homeDir, _ = os.UserHomeDir()
)

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
