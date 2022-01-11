package browser

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"hack-browser-data/pkg/browser/data"
)

var (
	// home dir path not for android and ios
	homeDir, _ = os.UserHomeDir()
)

func PickBrowsers(name string) {

}

func PickChromium(name string) []*chromium {
	var browsers []*chromium
	name = strings.ToLower(name)
	if name == "all" {
		for _, choice := range chromiumList {
			b, err := newChromium(choice.browserInfo, choice.items)
			if err != nil {
				panic(err)
			}
			browsers = append(browsers, b)
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

func PickFirefox(name string) []*firefox {
	var browsers []*firefox
	name = strings.ToLower(name)
	if name == "all" || name == "firefox" {
		for _, v := range firefoxList {
			b, err := newFirefox(v.browserInfo, v.items)
			if err != nil {
				panic(err)
			}
			// b := v.New(v.browserInfo, v.items)
			browsers = append(browsers, b...)
		}
		return browsers
	}
	// if choice, ok := browserList[name]; ok {
	// 	b := choice.New(choice.browserInfo, choice.items)
	// 	browsers = append(browsers, b)
	// 	return browsers
	// }
	return nil
}

type chromium struct {
	browserInfo *browserInfo
	items       []item
	itemPaths   map[item]string
}

func (c *chromium) GetProfilePath() string {
	return c.browserInfo.profilePath
}

func (c *chromium) GetStorageName() string {
	return c.browserInfo.storage
}

func (c *chromium) GetBrowserName() string {
	return c.browserInfo.name
}

type firefox struct {
	browserInfo    *browserInfo
	items          []item
	itemPaths      map[item]string
	multiItemPaths map[string]map[item]string
}

// NewBrowser 根据浏览器信息生成 Browser Interface
func newChromium(info *browserInfo, items []item) (*chromium, error) {
	c := &chromium{
		browserInfo: info,
		items:       items,
	}
	itemsPaths, err := getChromiumItemAbsPath(c.browserInfo.profilePath, c.items)
	if err != nil {
		return nil, err
	}
	c.itemPaths = itemsPaths
	return c, err
}

// newFirefox
func newFirefox(info *browserInfo, items []item) ([]*firefox, error) {
	f := &firefox{
		browserInfo: info,
		items:       items,
	}
	multiItemPaths, err := getFirefoxItemAbsPath(f.browserInfo.profilePath, f.items)
	if err != nil {
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
			// multiItemPaths: value,
		})
	}
	return firefoxList, nil
}

func getFirefoxItemAbsPath(profilePath string, items []item) (map[string]map[item]string, error) {
	var multiItemPaths = make(map[string]map[item]string)
	absProfilePath := path.Join(homeDir, filepath.Clean(profilePath))
	err := filepath.Walk(absProfilePath, firefoxWalkFunc(items, multiItemPaths))
	return multiItemPaths, err
}

func (f *firefox) copyItemFileToLocal() error {
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
	// for name, itemPaths := range f.multiItemPaths {
	// 	for item, path := range itemPaths {
	// 		var dstFilename = item.FileName()
	// 		locals, _ := filepath.Glob("*")
	// 		for _, v := range locals {
	// 			if v == dstFilename {
	// 				// TODO: Should Continue all iteration error
	// 				err := os.Remove(dstFilename)
	// 				if err != nil {
	// 					return err
	// 				}
	// 			}
	// 		}
	// 	}
	// 	// 	if v == dstFilename {
	// 	// 		err := os.Remove(dstFilename)
	// 	// 		if err != nil {
	// 	// 			return err
	// 	// 		}
	// 	// 	}
	// 	// }
	// 	//
	// 	// // TODO: Handle read file name error
	// 	// sourceFile, err := ioutil.ReadFile(sourcePath)
	// 	// if err != nil {
	// 	// 	fmt.Println(err.Error())
	// 	// }
	// 	// err = ioutil.WriteFile(dstFilename, sourceFile, 0777)
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// }
	// return nil
}

func firefoxWalkFunc(items []item, multiItemPaths map[string]map[item]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		for _, v := range items {
			if info.Name() == v.DefaultName() {
				parentDir := getParentDir(path)
				if _, exist := multiItemPaths[parentDir]; exist {
					multiItemPaths[parentDir][v] = path
				} else {
					multiItemPaths[parentDir] = map[item]string{v: path}
				}
			}
		}
		return err
	}
}

func getParentDir(absPath string) string {
	return filepath.Base(filepath.Dir(absPath))
}

func chromiumWalkFunc(items []item, itemPaths map[item]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		for _, item := range items {
			if item.DefaultName() == info.Name() && item == chromiumKey {
				itemPaths[item] = path
			}
			if item.DefaultName() == info.Name() && strings.Contains(path, "Default") {
				itemPaths[item] = path
			}
		}
		return err
	}
}

func getChromiumItemAbsPath(profilePath string, items []item) (map[item]string, error) {
	var itemPaths = make(map[item]string)
	absProfilePath := path.Join(homeDir, filepath.Clean(profilePath))
	err := filepath.Walk(absProfilePath, chromiumWalkFunc(items, itemPaths))
	return itemPaths, err
}

func (c *chromium) copyItemFileToLocal() error {
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

func (c *chromium) GetBrowsingData() []data.BrowsingData {
	var browsingData []data.BrowsingData
	for item := range c.itemPaths {
		if item != chromiumKey {
			d := item.NewBrowsingData()
			browsingData = append(browsingData, d)
		}
	}
	return browsingData
}

func (f *firefox) GetMasterKey() ([]byte, error) {
	return f.browserInfo.masterKey, nil
}

func (f *firefox) GetBrowserName() string {
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

type browserInfo struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
}

const (
	chromeName  = "Chrome"
	edgeName    = "Edge"
	firefoxName = "Firefox"
)

var defaultFirefoxItems = []item{
	firefoxKey4,
	firefoxPassword,
	firefoxCookie,
	firefoxBookmark,
	firefoxHistory,
	firefoxDownload,
	firefoxCreditCard,
	firefoxLocalStorage,
	firefoxExtension,
}

var defaultChromiumItems = []item{
	chromiumKey,
	chromiumPassword,
	chromiumCookie,
	chromiumBookmark,
	chromiumHistory,
	chromiumDownload,
	chromiumCreditCard,
	chromiumLocalStorage,
	chromiumExtension,
}
