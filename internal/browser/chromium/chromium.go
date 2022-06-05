package chromium

import (
	"io/fs"
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
func New(name, storage, profilePath string, items []item.Item) ([]*chromium, error) {
	c := &chromium{
		name:        name,
		storage:     storage,
		profilePath: profilePath,
		items:       items,
	}
	multiItemPaths, err := c.getMultiItemPath(c.profilePath, c.items)
	if err != nil {
		return nil, err
	}
	var chromiumList []*chromium
	for user, itemPaths := range multiItemPaths {
		chromiumList = append(chromiumList, &chromium{
			name:      fileutil.BrowserName(name, user),
			items:     typeutil.Keys(itemPaths),
			itemPaths: itemPaths,
			storage:   storage,
		})
	}
	return chromiumList, nil
}

func (c *chromium) Name() string {
	return c.name
}

func (c *chromium) BrowsingData() (*browingdata.Data, error) {
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
		filename := i.String()
		var err error
		switch {
		case fileutil.FolderExists(path):
			if i == item.ChromiumLocalStorage {
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

func (c *chromium) getItemPath(profilePath string, items []item.Item) (map[item.Item]string, error) {
	itemPaths := make(map[item.Item]string)
	parentDir := fileutil.ParentDir(profilePath)
	baseDir := fileutil.BaseDir(profilePath)
	err := filepath.Walk(parentDir, chromiumWalkFunc(items, itemPaths, baseDir))
	if err != nil {
		return itemPaths, err
	}
	fillLocalStoragePath(itemPaths, item.ChromiumLocalStorage)
	return itemPaths, nil
}

func (c *chromium) getMultiItemPath(profilePath string, items []item.Item) (map[string]map[item.Item]string, error) {
	// multiItemPaths is a map of user to item path, map[profile 1][item's name & path key pair]
	multiItemPaths := make(map[string]map[item.Item]string)
	parentDir := fileutil.ParentDir(profilePath)
	err := filepath.Walk(parentDir, chromiumWalkFunc2(items, multiItemPaths))
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

func chromiumWalkFunc2(items []item.Item, multiItemPaths map[string]map[item.Item]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		for _, v := range items {
			if info.Name() == v.FileName() {
				parentBaseDir := fileutil.ParentBaseDir(path)
				if parentBaseDir == "System Profile" {
					continue
				}
				if _, exist := multiItemPaths[parentBaseDir]; exist {
					multiItemPaths[parentBaseDir][v] = path
				} else {
					multiItemPaths[parentBaseDir] = map[item.Item]string{v: path}
				}
			}
		}
		return err
	}
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

func fillLocalStoragePath(itemPaths map[item.Item]string, storage item.Item) {
	if p, ok := itemPaths[item.ChromiumHistory]; ok {
		lsp := filepath.Join(filepath.Dir(p), storage.FileName())
		if fileutil.FolderExists(lsp) {
			itemPaths[item.ChromiumLocalStorage] = lsp
		}
	}
}
