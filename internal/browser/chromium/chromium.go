package chromium

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"hack-browser-data/internal/browingdata"
	"hack-browser-data/internal/item"
	"hack-browser-data/internal/utils/fileutil"
	"hack-browser-data/internal/utils/typeutil"
)

type chromium struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	items       []item.Item
	itemPaths   map[item.Item]string
}

// New creates a new instance of chromium browser, fill item's path if item is exist.
func New(name, storage, profilePath string, items []item.Item) (*chromium, error) {

	// TODO: Handle file path is not exist
	if !fileutil.FolderExists(profilePath) {
		return nil, fmt.Errorf("%s profile path is not exist: %s", name, profilePath)
	}
	itemsPaths, err := getChromiumItemPath(profilePath, items)
	if err != nil {
		return nil, err
	}

	c := &chromium{
		name:        name,
		storage:     storage,
		profilePath: profilePath,
		items:       typeutil.Keys(itemsPaths),
		itemPaths:   itemsPaths,
	}
	// new browsing data
	return c, err
}

func (c *chromium) GetName() string {
	return c.name
}

func (c *chromium) GetBrowsingData() []browingdata.Source {
	var browsingData []browingdata.Source
	data := browingdata.New(c.items)
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
		var dstFilename = item.TempName()
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
			case it.FileName() == info.Name():
				if it == item.ChromiumKey {
					itemPaths[it] = path
				}
				// TODO: Handle file path is not in Default folder
				if strings.Contains(path, "Default") {
					itemPaths[it] = path
				}
			}
		}
		return err
	}
}
