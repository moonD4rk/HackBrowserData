package chromium

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browserdata"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

type Chromium struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	dataTypes   []types.DataType
	Paths       map[types.DataType]string
}

// New create instance of Chromium browser, fill item's path if item is existed.
func New(name, storage, profilePath string, dataTypes []types.DataType) ([]*Chromium, error) {
	c := &Chromium{
		name:        name,
		storage:     storage,
		profilePath: profilePath,
		dataTypes:   dataTypes,
	}
	multiDataTypePaths, err := c.userDataTypePaths(c.profilePath, c.dataTypes)
	if err != nil {
		return nil, err
	}
	chromiumList := make([]*Chromium, 0, len(multiDataTypePaths))
	for user, itemPaths := range multiDataTypePaths {
		chromiumList = append(chromiumList, &Chromium{
			name:      fileutil.BrowserName(name, user),
			dataTypes: typeutil.Keys(itemPaths),
			Paths:     itemPaths,
			storage:   storage,
		})
	}
	return chromiumList, nil
}

func (c *Chromium) Name() string {
	return c.name
}

func (c *Chromium) BrowsingData(isFullExport bool) (*browserdata.BrowserData, error) {
	// delete chromiumKey from dataTypes, doesn't need to export key
	var dataTypes []types.DataType
	for _, dt := range c.dataTypes {
		if dt != types.ChromiumKey {
			dataTypes = append(dataTypes, dt)
		}
	}

	if !isFullExport {
		dataTypes = types.FilterSensitiveItems(c.dataTypes)
	}

	data := browserdata.New(dataTypes)

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
	for i, path := range c.Paths {
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
			log.Errorf("copy item to local, path %s, filename %s err %v", path, filename, err)
			continue
		}
	}
	return nil
}

// userDataTypePaths return a map of user to item path, map[profile 1][item's name & path key pair]
func (c *Chromium) userDataTypePaths(profilePath string, items []types.DataType) (map[string]map[types.DataType]string, error) {
	multiItemPaths := make(map[string]map[types.DataType]string)
	parentDir := fileutil.ParentDir(profilePath)
	err := filepath.Walk(parentDir, chromiumWalkFunc(items, multiItemPaths))
	if err != nil {
		return nil, err
	}
	var keyPath string
	var dir string
	for userDir, profiles := range multiItemPaths {
		for _, profile := range profiles {
			if strings.HasSuffix(profile, types.ChromiumKey.Filename()) {
				keyPath = profile
				dir = userDir
				break
			}
		}
	}
	t := make(map[string]map[types.DataType]string)
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
func chromiumWalkFunc(items []types.DataType, multiItemPaths map[string]map[types.DataType]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Warnf("skipping walk chromium path permission error, path %s, err %v", path, err)
				return nil
			}
			return err
		}
		for _, v := range items {
			if info.Name() != v.Filename() {
				continue
			}
			if strings.Contains(path, "System Profile") {
				continue
			}
			if strings.Contains(path, "Snapshot") {
				continue
			}
			profileFolder := fileutil.ParentBaseDir(path)
			if strings.Contains(filepath.ToSlash(path), "/Network/Cookies") {
				profileFolder = fileutil.BaseDir(strings.ReplaceAll(filepath.ToSlash(path), "/Network/Cookies", ""))
			}
			if _, exist := multiItemPaths[profileFolder]; exist {
				multiItemPaths[profileFolder][v] = path
			} else {
				multiItemPaths[profileFolder] = map[types.DataType]string{v: path}
			}
		}
		return nil
	}
}

func fillLocalStoragePath(itemPaths map[types.DataType]string, storage types.DataType) {
	if p, ok := itemPaths[types.ChromiumHistory]; ok {
		lsp := filepath.Join(filepath.Dir(p), storage.Filename())
		if fileutil.IsDirExists(lsp) {
			itemPaths[types.ChromiumLocalStorage] = lsp
		}
	}
}
