package firefox

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // sqlite3 driver TODO: replace with chooseable driver

	"github.com/moond4rk/hackbrowserdata/browserdata"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

type Firefox struct {
	name        string
	storage     string
	profilePath string
	masterKey   []byte
	items       []types.DataType
	itemPaths   map[types.DataType]string
}

var ErrProfilePathNotFound = errors.New("profile path not found")

// New returns new Firefox instances.
func New(profilePath string, items []types.DataType) ([]*Firefox, error) {
	multiItemPaths := make(map[string]map[types.DataType]string)
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
		filename := i.TempFilename()
		if err := fileutil.CopyFile(path, filename); err != nil {
			return err
		}
	}
	return nil
}

func firefoxWalkFunc(items []types.DataType, multiItemPaths map[string]map[types.DataType]string) fs.WalkDirFunc {
	return func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Warnf("skipping walk firefox path %s permission error: %v", path, err)
				return nil
			}
			return err
		}
		for _, v := range items {
			if info.Name() == v.Filename() {
				parentBaseDir := fileutil.ParentBaseDir(path)
				if _, exist := multiItemPaths[parentBaseDir]; exist {
					multiItemPaths[parentBaseDir][v] = path
				} else {
					multiItemPaths[parentBaseDir] = map[types.DataType]string{v: path}
				}
			}
		}

		return nil
	}
}

// GetMasterKey returns master key of Firefox. from key4.db
func (f *Firefox) GetMasterKey() ([]byte, error) {
	tempFilename := types.FirefoxKey4.TempFilename()

	// Open and defer close of the database.
	keyDB, err := sql.Open("sqlite", tempFilename)
	if err != nil {
		return nil, fmt.Errorf("open key4.db error: %w", err)
	}
	defer os.Remove(tempFilename)
	defer keyDB.Close()

	metaItem1, metaItem2, err := queryMetaData(keyDB)
	if err != nil {
		return nil, fmt.Errorf("query metadata error: %w", err)
	}

	candidates, err := queryNssPrivateCandidates(keyDB)
	if err != nil {
		return nil, fmt.Errorf("query NSS private error: %w", err)
	}
	loginCipherPairs, _ := getFirefoxLoginCipherPairs()

	var (
		fallbackKey []byte
		lastErr     error
	)
	for _, c := range candidates {
		masterKey, err := processMasterKey(metaItem1, metaItem2, c.a11, c.a102)
		if err != nil {
			lastErr = err
			continue
		}
		if fallbackKey == nil {
			fallbackKey = masterKey
		}

		if len(loginCipherPairs) == 0 {
			return masterKey, nil
		}
		if canDecryptAnyLoginCipherPair(masterKey, loginCipherPairs) {
			return masterKey, nil
		}
	}

	if fallbackKey != nil {
		return fallbackKey, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no valid firefox master key found in nssPrivate")
}

// getFirefoxLoginCipherPairs reads login cipher pairs from the old temp file path.
// Used by the old architecture (GetMasterKey); new code uses parseLoginCipherPairs / validateKeyWithLogins.
func getFirefoxLoginCipherPairs() ([]loginCipherPair, error) {
	raw, err := os.ReadFile(types.FirefoxPassword.TempFilename())
	if err != nil {
		return nil, err
	}
	return parseLoginCipherPairs(raw)
}

func (f *Firefox) Name() string {
	return f.name
}

func (f *Firefox) BrowsingData(isFullExport bool) (*browserdata.BrowserData, error) {
	dataTypes := f.items
	if !isFullExport {
		dataTypes = types.FilterSensitiveItems(f.items)
	}

	data := browserdata.New(dataTypes)

	if err := f.copyItemToLocal(); err != nil {
		return nil, err
	}

	masterKey, err := f.GetMasterKey()
	if err != nil {
		return nil, err
	}

	f.masterKey = masterKey
	if err := data.Recovery(f.masterKey); err != nil {
		return nil, err
	}
	return data, nil
}
