package browser

import (
	"os"
	"strings"

	"hack-browser-data/internal/browingdata"
	"hack-browser-data/internal/browser/chromium"
	"hack-browser-data/internal/browser/firefox"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils/fileutil"
	"hack-browser-data/internal/utils/typeutil"
)

type Browser interface {
	// Name is browser's name
	Name() string
	// BrowsingData returns all browsing data in the browser.
	BrowsingData() (*browingdata.Data, error)
}

func PickBrowser(name, profile string) ([]Browser, error) {
	var browsers []Browser
	clist := pickChromium(name, profile)
	for _, b := range clist {
		if b != nil {
			browsers = append(browsers, b)
		}
	}
	flist := pickFirefox(name, profile)
	for _, b := range flist {
		if b != nil {
			browsers = append(browsers, b)
		}
	}
	return browsers, nil
}

func pickChromium(name, profile string) []Browser {
	var browsers []Browser
	name = strings.ToLower(name)
	// TODO: add support for 「all」 flag and set profilePath
	if name == "all" {
		for _, v := range chromiumList {
			if b, err := chromium.New(v.name, v.storage, v.profilePath, v.items); err == nil {
				log.Noticef("find browser %s success", b.Name())
				browsers = append(browsers, b)
			} else {
				// TODO: show which browser find failed
				if strings.Contains(err.Error(), "profile folder is not exist") {
					log.Errorf("find browser %s failed, profile folder is not exist, maybe not installed", v.name)
					continue
				} else {
					log.Errorf("new chromium error:", err)
				}
			}
		}
	}
	if c, ok := chromiumList[name]; ok {
		if profile == "" {
			profile = c.profilePath
		}
		b, err := chromium.New(c.name, c.storage, profile, c.items)
		if err != nil {
			if strings.Contains(err.Error(), "profile folder is not exist") {
				log.Fatalf("find browser %s failed, profile folder is not exist, maybe not installed", c.name)
			} else {
				log.Fatalf("new chromium error:", err)
			}
		}
		browsers = append(browsers, b)
	}
	return browsers
}

func pickFirefox(name, profile string) []Browser {
	var browsers []Browser
	name = strings.ToLower(name)
	if name == "all" || name == "firefox" {
		for _, v := range firefoxList {
			if profile == "" {
				profile = v.profilePath
			} else {
				profile = fileutil.ParentDir(profile)
			}
			if multiFirefox, err := firefox.New(v.name, v.storage, profile, v.items); err == nil {
				for _, b := range multiFirefox {
					log.Noticef("find browser firefox %s success", b.Name())
					browsers = append(browsers, b)
				}
			} else {
				if strings.Contains(err.Error(), "profile folder is not exist") {
					log.Errorf("find browser firefox %s failed, profile folder is not exist", v.name)
				} else {
					log.Error(err)
				}
			}

		}
		return browsers
	}
	return nil
}

func ListBrowser() []string {
	var l []string
	l = append(l, typeutil.Keys(chromiumList)...)
	l = append(l, typeutil.Keys(firefoxList)...)
	return l
}

var (
	// home dir path for all platforms
	homeDir, _ = os.UserHomeDir()
)

const (
	chromeName     = "Chrome"
	chromeBetaName = "Chrome Beta"
	chromiumName   = "Chromium"
	edgeName       = "Microsoft Edge"
	speed360Name   = "360speed"
	qqBrowserName  = "QQ"
	braveName      = "Brave"
	operaName      = "Opera"
	operaGXName    = "OperaGX"
	vivaldiName    = "Vivaldi"
	coccocName     = "CocCoc"
	yandexName     = "Yandex"

	firefoxName        = "Firefox"
	firefoxBetaName    = "Firefox Beta"
	firefoxDevName     = "Firefox Dev"
	firefoxNightlyName = "Firefox Nightly"
	firefoxESRName     = "Firefox ESR"
)
