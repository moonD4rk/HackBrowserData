package chromium

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/moond4rk/HackBrowserData/browingdata"
	"github.com/moond4rk/HackBrowserData/item"
	"github.com/moond4rk/HackBrowserData/utils/fileutil"
	"github.com/moond4rk/HackBrowserData/utils/typeutil"
)

type Chromium struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	items       []item.Item
	itemPaths   map[item.Item]string
}

// New create instance of Chromium browser, fill item's path if item is existed.
func New(name, storage, profilePath string, items []item.Item) ([]*Chromium, error) {
	c := &Chromium{
		name:        name,
		storage:     storage,
		profilePath: profilePath,
		items:       items,
	}
	multiItemPaths, err := c.userItemPaths(c.profilePath, c.items)
	if err != nil {
		return nil, err
	}
	chromiumList := make([]*Chromium, 0, len(multiItemPaths))
	for user, itemPaths := range multiItemPaths {
		chromiumList = append(chromiumList, &Chromium{
			name:      fileutil.BrowserName(name, user),
			items:     typeutil.Keys(itemPaths),
			itemPaths: itemPaths,
			storage:   storage,
		})
	}
	return chromiumList, nil
}

func (c *Chromium) Name() string {
	return c.name
}

func (c *Chromium) BrowsingData(isFullExport bool) (*browingdata.Data, error) {
	items := c.items
	if !isFullExport {
		items = item.FilterSensitiveItems(c.items)
	}

	data := browingdata.New(items)

	if err := c.copyItemToLocal(); err != nil {
		return nil, err
	}

	masterKey, err := c.GetMasterKey()
	if err != nil {
		return nil, err
	}

	c.masterKey = masterKey
	if err := data.Recovery(c.masterKey); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Chromium) copyItemToLocal() error {
	for i, path := range c.itemPaths {
		filename := i.String()
		var err error
		switch {
		case fileutil.IsDirExists(path):
			if i == item.ChromiumLocalStorage {
				err = fileutil.CopyDir(path, filename, "lock")
			}
			if i == item.ChromiumSessionStorage {
				err = fileutil.CopyDir(path, filename, "lock")
			}
			if i == item.ChromiumExtension {
				err = fileutil.CopyDirHasSuffix(path, filename, "manifest.json")
			}
		default:
			err = fileutil.CopyFile(path, filename)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// userItemPaths return a map of user to item path, map[profile 1][item's name & path key pair]
func (c *Chromium) userItemPaths(profilePath string, items []item.Item) (map[string]map[item.Item]string, error) {
	multiItemPaths := make(map[string]map[item.Item]string)
	parentDir := fileutil.ParentDir(profilePath)
	err := filepath.Walk(parentDir, chromiumWalkFunc(items, multiItemPaths))
	if err != nil {
		return nil, err
	}
	var keyPath string
	var dir string
	for userDir, v := range multiItemPaths {
		for _, p := range v {
			if strings.HasSuffix(p, item.ChromiumKey.FileName()) {
				keyPath = p
				dir = userDir
				break
			}
		}
	}
	t := make(map[string]map[item.Item]string)
	for userDir, v := range multiItemPaths {
		if userDir == dir {
			continue
		}
		t[userDir] = v
		t[userDir][item.ChromiumKey] = keyPath
		fillLocalStoragePath(t[userDir], item.ChromiumLocalStorage)
	}
	return t, nil
}

// chromiumWalkFunc return a filepath.WalkFunc to find item's path
func chromiumWalkFunc(items []item.Item, multiItemPaths map[string]map[item.Item]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		for _, v := range items {
			if info.Name() == v.FileName() {
				if strings.Contains(path, "System Profile") {
					continue
				}
				profileFolder := fileutil.ParentBaseDir(path)
				if strings.Contains(filepath.ToSlash(path), "/Network/Cookies") {
					profileFolder = fileutil.BaseDir(strings.ReplaceAll(filepath.ToSlash(path), "/Network/Cookies", ""))
				}
				if _, exist := multiItemPaths[profileFolder]; exist {
					multiItemPaths[profileFolder][v] = path
				} else {
					multiItemPaths[profileFolder] = map[item.Item]string{v: path}
				}
			}
		}
		return err
	}
}

func fillLocalStoragePath(itemPaths map[item.Item]string, storage item.Item) {
	if p, ok := itemPaths[item.ChromiumHistory]; ok {
		lsp := filepath.Join(filepath.Dir(p), storage.FileName())
		if fileutil.IsDirExists(lsp) {
			itemPaths[item.ChromiumLocalStorage] = lsp
		}
	}
}
