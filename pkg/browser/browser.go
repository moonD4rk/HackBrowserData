package browser

import (
	"fmt"
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

func PickBrowsers(name string) []*chromium {
	var browsers []*chromium
	name = strings.ToLower(name)
	if name == "all" {
		for _, v := range browserList {
			b := v.New(v.browserInfo, v.items)
			browsers = append(browsers, b)
		}
		return browsers
	}
	if choice, ok := browserList[name]; ok {
		b := choice.New(choice.browserInfo, choice.items)
		browsers = append(browsers, b)
		return browsers
	}
	return nil
}

type chromium struct {
	browserInfo *browserInfo
	items       []item
	itemPaths   map[item]string
	masterKey   []byte
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
	browserInfo *browserInfo
	items       []item
	itemPaths   map[item]string
	masterKey   []byte
}

// NewBrowser 根据浏览器信息生成 Browser Interface
func newBrowser(browserInfo *browserInfo, items []item) *chromium {
	return &chromium{
		browserInfo: browserInfo,
		items:       items,
	}
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

func (c *chromium) walkItemAbsPath() error {
	var itemPaths = make(map[item]string)
	absProfilePath := path.Join(homeDir, filepath.Clean(c.browserInfo.profilePath))
	err := filepath.Walk(absProfilePath, chromiumWalkFunc(defaultChromiumItems, itemPaths))
	c.itemPaths = itemPaths
	return err
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

type browserInfo struct {
	name        string
	storage     string
	profilePath string
	masterKey   string
}

const (
	chromeName = "Chrome"
	edgeName   = "Edge"
)

var defaultChromiumItems = []item{
	chromiumKey,
	chromiumPassword,
	chromiumCookie,
	chromiumBookmark,
	chromiumHistory,
	chromiumDownload,
	chromiumCreditcard,
	chromiumLocalStorage,
	chromiumExtension,
}
