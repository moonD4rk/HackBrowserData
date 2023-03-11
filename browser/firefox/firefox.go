package firefox

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/moond4rk/HackBrowserData/browingdata"
	"github.com/moond4rk/HackBrowserData/item"
	"github.com/moond4rk/HackBrowserData/utils/fileutil"
	"github.com/moond4rk/HackBrowserData/utils/typeutil"
)

type Firefox struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	items       []item.Item
	itemPaths   map[item.Item]string
}

var ErrProfilePathNotFound = errors.New("profile path not found")

// New returns a new Firefox instance.
func New(name, storage, profilePath string, items []item.Item) ([]*Firefox, error) {
	f := &Firefox{
		name:        name,
		storage:     storage,
		profilePath: profilePath,
		items:       items,
	}
	multiItemPaths, err := f.getMultiItemPath(f.profilePath, f.items)
	if err != nil {
		return nil, err
	}

	firefoxList := make([]*Firefox, 0, len(multiItemPaths))
	for name, itemPaths := range multiItemPaths {
		firefoxList = append(firefoxList, &Firefox{
			name:      fmt.Sprintf("firefox-%s", name),
			items:     typeutil.Keys(itemPaths),
			itemPaths: itemPaths,
		})
	}
	return firefoxList, nil
}

func (f *Firefox) getMultiItemPath(profilePath string, items []item.Item) (map[string]map[item.Item]string, error) {
	multiItemPaths := make(map[string]map[item.Item]string)
	err := filepath.Walk(profilePath, firefoxWalkFunc(items, multiItemPaths))
	return multiItemPaths, err
}

func (f *Firefox) copyItemToLocal() error {
	for i, path := range f.itemPaths {
		filename := i.String()
		if err := fileutil.CopyFile(path, filename); err != nil {
			return err
		}
	}
	return nil
}

func firefoxWalkFunc(items []item.Item, multiItemPaths map[string]map[item.Item]string) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		for _, v := range items {
			if info.Name() == v.FileName() {
				parentBaseDir := fileutil.ParentBaseDir(path)
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

func (f *Firefox) GetMasterKey() ([]byte, error) {
	return f.masterKey, nil
}

func (f *Firefox) Name() string {
	return f.name
}

func (f *Firefox) BrowsingData(isFullExport bool) (*browingdata.Data, error) {
	items := f.items
	if !isFullExport {
		items = item.FilterSensitiveItems(f.items)
	}

	b := browingdata.New(items)

	if err := f.copyItemToLocal(); err != nil {
		return nil, err
	}

	masterKey, err := f.GetMasterKey()
	if err != nil {
		return nil, err
	}

	f.masterKey = masterKey
	if err := b.Recovery(f.masterKey); err != nil {
		return nil, err
	}
	return b, nil
}
