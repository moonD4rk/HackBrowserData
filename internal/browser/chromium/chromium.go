package chromium

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"hack-browser-data/internal/browser/data"
	"hack-browser-data/internal/browser/item"
)

type chromium struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	items       []item.Item
	itemPaths   map[item.Item]string
}

// newChromium 根据浏览器信息生成 Browser Interface
func newChromium(name, storage, profilePath string, items []item.Item) (*chromium, error) {
	c := &chromium{
		name:        name,
		storage:     storage,
		profilePath: profilePath,
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
	return c.name
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

func getChromiumItemPath(profilePath string, items []item.Item) (map[item.Item]string, error) {
	var itemPaths = make(map[item.Item]string)
	err := filepath.Walk(profilePath, chromiumWalkFunc(items, itemPaths))
	return itemPaths, err
}

func chromiumWalkFunc(items []item.Item, itemPaths map[item.Item]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		for _, it := range items {
			switch {
			case it.DefaultName() == info.Name():
				if it == it.chromiumKey {
					itemPaths[it] = path
				}
				if strings.Contains(path, "Default") {
					itemPaths[it] = path
				}
			}
		}
		return err
	}
}
