package provider

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hack-browser-data/internal/browser"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/provider/chromium"
	"hack-browser-data/internal/provider/firefox"
	"hack-browser-data/internal/utils/fileutil"
	"hack-browser-data/internal/utils/typeutil"
)

func PickBrowsers(name, profile string) ([]browser.Browser, error) {
	var browsers []browser.Browser
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

func pickChromium(name, profile string) []browser.Browser {
	var browsers []browser.Browser
	name = strings.ToLower(name)
	if name == "all" {
		for _, v := range chromiumList {
			if !fileutil.FolderExists(filepath.Clean(v.profilePath)) {
				log.Noticef("find browser %s failed, profile folder does not exist", v.name)
				continue
			}
			if multiChromium, err := chromium.New(v.name, v.storage, v.profilePath, v.items); err == nil {
				log.Noticef("find browser %s success", v.name)
				for _, b := range multiChromium {
					log.Noticef("find browser %s success", b.Name())
					browsers = append(browsers, b)
				}
			} else {
				log.Errorf("new chromium error: %s", err.Error())
			}
		}
	}
	if c, ok := chromiumList[name]; ok {
		if profile == "" {
			profile = c.profilePath
		}
		if !fileutil.FolderExists(filepath.Clean(profile)) {
			log.Fatalf("find browser %s failed, profile folder does not exist", c.name)
		}
		chromiumList, err := chromium.New(c.name, c.storage, profile, c.items)
		if err != nil {
			log.Fatalf("new chromium error: %s", err)
		}
		for _, b := range chromiumList {
			log.Noticef("find browser %s success", b.Name())
			browsers = append(browsers, b)
		}
	}
	return browsers
}

func pickFirefox(name, profile string) []browser.Browser {
	var browsers []browser.Browser
	name = strings.ToLower(name)
	if name == "all" || name == "firefox" {
		for _, v := range firefoxList {
			if profile == "" {
				profile = v.profilePath
			} else {
				profile = fileutil.ParentDir(profile)
			}
			if !fileutil.FolderExists(filepath.Clean(profile)) {
				log.Noticef("find browser firefox %s failed, profile folder does not exist", v.name)
				continue
			}
			if multiFirefox, err := firefox.New(v.name, v.storage, profile, v.items); err == nil {
				for _, b := range multiFirefox {
					log.Noticef("find browser firefox %s success", b.Name())
					browsers = append(browsers, b)
				}
			} else {
				log.Error(err)
			}
		}
		return browsers
	}
	return nil
}

func ListBrowsers() []string {
	var l []string
	l = append(l, typeutil.Keys(chromiumList)...)
	l = append(l, typeutil.Keys(firefoxList)...)
	sort.Strings(l)
	return l
}

// home dir path for all platforms
var homeDir, _ = os.UserHomeDir()

const (
	chromeName     = "Chrome"
	chromeBetaName = "Chrome Beta"
	chromiumName   = "Chromium"
	edgeName       = "Microsoft Edge"
	braveName      = "Brave"
	operaName      = "Opera"
	operaGXName    = "OperaGX"
	vivaldiName    = "Vivaldi"
	coccocName     = "CocCoc"
	yandexName     = "Yandex"
	firefoxName    = "Firefox"
	speed360Name   = "360speed"
	qqBrowserName  = "QQ"
)
