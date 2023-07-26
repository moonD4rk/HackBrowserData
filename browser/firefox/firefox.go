package firefox

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/browsingdata"
	"github.com/moond4rk/hackbrowserdata/item"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
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

// New returns new Firefox instances.
func New(profilePath string, items []item.Item) ([]*Firefox, error) {
	multiItemPaths := make(map[string]map[item.Item]string)
	// ignore walk dir error since it can be produced by a single entry
	_ = filepath.WalkDir(profilePath, firefoxWalkFunc(items, multiItemPaths))

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

func (f *Firefox) copyItemToLocal() error {
	for i, path := range f.itemPaths {
		filename := i.String()
		if err := fileutil.CopyFile(path, filename); err != nil {
			return err
		}
	}
	return nil
}

func firefoxWalkFunc(items []item.Item, multiItemPaths map[string]map[item.Item]string) fs.WalkDirFunc {
	return func(path string, info fs.DirEntry, err error) error {
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

func (f *Firefox) BrowsingData(isFullExport bool) (*browsingdata.Data, error) {
	items := f.items
	if !isFullExport {
		items = item.FilterSensitiveItems(f.items)
	}

	b := browsingdata.New(items)

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
