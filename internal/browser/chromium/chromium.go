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

// New create instance of chromium browser, fill item's path if item is existed.
func New(name, storage, profilePath string, items []item.Item) (*chromium, error) {
	c := &chromium{
		name:    name,
		storage: storage,
	}
	// TODO: Handle file path is not exist
	if !fileutil.FolderExists(profilePath) {
		return nil, fmt.Errorf("%s profile path is not exist: %s", name, profilePath)
	}
	itemsPaths, err := c.getItemPath(profilePath, items)
	if err != nil {
		return nil, err
	}
	c.profilePath = profilePath
	c.itemPaths = itemsPaths
	c.items = typeutil.Keys(itemsPaths)
	return c, err
}

func (c *chromium) Name() string {
	return c.name
}

func (c *chromium) GetBrowsingData() (*browingdata.Data, error) {
	b := browingdata.New(c.items)

	if err := c.copyItemToLocal(); err != nil {
		return nil, err
	}

	masterKey, err := c.GetMasterKey()
	if err != nil {
		return nil, err
	}

	c.masterKey = masterKey
	if err := b.Recovery(c.masterKey); err != nil {
		return nil, err
	}
	return b, nil
}

func (c *chromium) copyItemToLocal() error {
	for i, path := range c.itemPaths {
		// var dstFilename = item.TempName()
		var filename = i.String()
		// TODO: Handle read file error
		d, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Println(err.Error())
		}
		err = ioutil.WriteFile(filename, d, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *chromium) getItemPath(profilePath string, items []item.Item) (map[item.Item]string, error) {
	var itemPaths = make(map[item.Item]string)
	parentDir := fileutil.ParentDir(profilePath)
	baseDir := fileutil.BaseDir(profilePath)
	err := filepath.Walk(parentDir, chromiumWalkFunc(items, itemPaths, baseDir))
	return itemPaths, err
}

func chromiumWalkFunc(items []item.Item, itemPaths map[item.Item]string, baseDir string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		for _, it := range items {
			switch {
			case it.FileName() == info.Name():
				if it == item.ChromiumKey {
					itemPaths[it] = path
				}
				if strings.Contains(path, baseDir) {
					itemPaths[it] = path
				}
			}
		}
		return err
	}
}
