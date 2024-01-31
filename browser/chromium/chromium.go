package chromium

import (
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browserdata"
	"github.com/moond4rk/hackbrowserdata/browserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

type Chromium struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	items       []types.BrowserDataType
	itemPaths   map[types.BrowserDataType]string
}

// New create instance of Chromium browser, fill item's path if item is existed.
func New(name, storage, profilePath string, items []types.BrowserDataType) ([]*Chromium, error) {
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

func (c *Chromium) BrowsingData(isFullExport bool) (*browserdata.Data, error) {
	items := c.items
	if !isFullExport {
		items = types.FilterSensitiveItems(c.items)
	}

	data := browserdata.New(items)

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
		filename := i.TempFilename()
		var err error
		switch {
		case fileutil.IsDirExists(path):
			if i == types.ChromiumLocalStorage {
				err = fileutil.CopyDir(path, filename, "lock")
			}
			if i == types.ChromiumSessionStorage {
				err = fileutil.CopyDir(path, filename, "lock")
			}
		default:
			err = fileutil.CopyFile(path, filename)
		}
		if err != nil {
			slog.Error("copy item to local error", "path", path, "filename", filename, "err", err)
			continue
		}
	}
	return nil
}

// userItemPaths return a map of user to item path, map[profile 1][item's name & path key pair]
func (c *Chromium) userItemPaths(profilePath string, items []types.BrowserDataType) (map[string]map[types.BrowserDataType]string, error) {
	multiItemPaths := make(map[string]map[types.BrowserDataType]string)
	parentDir := fileutil.ParentDir(profilePath)
	err := filepath.Walk(parentDir, chromiumWalkFunc(items, multiItemPaths))
	if err != nil {
		return nil, err
	}
	var keyPath string
	var dir string
	for userDir, v := range multiItemPaths {
		for _, p := range v {
			if strings.HasSuffix(p, types.ChromiumKey.Filename()) {
				keyPath = p
				dir = userDir
				break
			}
		}
	}
	t := make(map[string]map[types.BrowserDataType]string)
	for userDir, v := range multiItemPaths {
		if userDir == dir {
			continue
		}
		t[userDir] = v
		t[userDir][types.ChromiumKey] = keyPath
		fillLocalStoragePath(t[userDir], types.ChromiumLocalStorage)
	}
	return t, nil
}

// chromiumWalkFunc return a filepath.WalkFunc to find item's path
func chromiumWalkFunc(items []types.BrowserDataType, multiItemPaths map[string]map[types.BrowserDataType]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		for _, v := range items {
			if info.Name() != v.Filename() {
				continue
			}
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
				multiItemPaths[profileFolder] = map[types.BrowserDataType]string{v: path}
			}
		}
		return err
	}
}

func fillLocalStoragePath(itemPaths map[types.BrowserDataType]string, storage types.BrowserDataType) {
	if p, ok := itemPaths[types.ChromiumHistory]; ok {
		lsp := filepath.Join(filepath.Dir(p), storage.Filename())
		if fileutil.IsDirExists(lsp) {
			itemPaths[types.ChromiumLocalStorage] = lsp
		}
	}
}
