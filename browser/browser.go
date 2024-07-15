package browser

import (
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/browser/firefox"
	"github.com/moond4rk/hackbrowserdata/browserdata"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

type Browser interface {
	// Name is browser's name
	Name() string
	// BrowsingData returns all browsing data in the browser.
	BrowsingData(isFullExport bool) (*browserdata.BrowserData, error)
}

// PickBrowsers returns a list of browsers that match the name and profile.
func PickBrowsers(name, profile string) ([]Browser, error) {
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
	if name == "all" {
		for _, v := range chromiumList {
			if !fileutil.IsDirExists(filepath.Clean(v.profilePath)) {
				slog.Warn("find browser failed, profile folder does not exist", "browser", v.name)
				continue
			}
			multiChromium, err := chromium.New(v.name, v.storage, v.profilePath, v.dataTypes)
			if err != nil {
				slog.Error("new chromium error", "err", err)
				continue
			}
			for _, b := range multiChromium {
				slog.Warn("find browser success", "browser", b.Name())
				browsers = append(browsers, b)
			}
		}
	}
	if c, ok := chromiumList[name]; ok {
		if profile == "" {
			profile = c.profilePath
		}
		if !fileutil.IsDirExists(filepath.Clean(profile)) {
			slog.Error("find browser failed, profile folder does not exist", "browser", c.name)
		}
		chromes, err := chromium.New(c.name, c.storage, profile, c.dataTypes)
		if err != nil {
			slog.Error("new chromium error", "err", err)
		}
		for _, chrome := range chromes {
			slog.Warn("find browser success", "browser", chrome.Name())
			browsers = append(browsers, chrome)
		}
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

			if !fileutil.IsDirExists(filepath.Clean(profile)) {
				slog.Warn("find browser failed, profile folder does not exist", "browser", v.name)
				continue
			}

			if multiFirefox, err := firefox.New(profile, v.dataTypes); err == nil {
				for _, b := range multiFirefox {
					slog.Warn("find browser success", "browser", b.Name())
					browsers = append(browsers, b)
				}
			} else {
				slog.Error("new firefox error", "err", err)
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

func Names() string {
	return strings.Join(ListBrowsers(), "|")
}
