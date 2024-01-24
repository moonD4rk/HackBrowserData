package firefox

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // sqlite3 driver TODO: replace with chooseable driver

	"github.com/moond4rk/hackbrowserdata/browsingdata"
	"github.com/moond4rk/hackbrowserdata/crypto"
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
		filename := i.TempFilename()
		if err := fileutil.CopyFile(path, filename); err != nil {
			return err
		}
	}
	return nil
}

func firefoxWalkFunc(items []item.Item, multiItemPaths map[string]map[item.Item]string) fs.WalkDirFunc {
	return func(path string, info fs.DirEntry, err error) error {
		for _, v := range items {
			if info.Name() == v.Filename() {
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

// GetMasterKey returns master key of Firefox. from key4.db
func (f *Firefox) GetMasterKey() ([]byte, error) {
	tempFilename := item.FirefoxKey4.TempFilename()

	// Open and defer close of the database.
	keyDB, err := sql.Open("sqlite", tempFilename)
	if err != nil {
		return nil, fmt.Errorf("open key4.db error: %w", err)
	}
	defer os.Remove(tempFilename)
	defer keyDB.Close()

	globalSalt, metaBytes, err := queryMetaData(keyDB)
	if err != nil {
		return nil, fmt.Errorf("query metadata error: %w", err)
	}

	nssA11, nssA102, err := queryNssPrivate(keyDB)
	if err != nil {
		return nil, fmt.Errorf("query NSS private error: %w", err)
	}

	return processMasterKey(globalSalt, metaBytes, nssA11, nssA102)
}

func queryMetaData(db *sql.DB) ([]byte, []byte, error) {
	const query = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	var globalSalt, metaBytes []byte
	if err := db.QueryRow(query).Scan(&globalSalt, &metaBytes); err != nil {
		return nil, nil, err
	}
	return globalSalt, metaBytes, nil
}

func queryNssPrivate(db *sql.DB) ([]byte, []byte, error) {
	const query = `SELECT a11, a102 from nssPrivate`
	var nssA11, nssA102 []byte
	if err := db.QueryRow(query).Scan(&nssA11, &nssA102); err != nil {
		return nil, nil, err
	}
	return nssA11, nssA102, nil
}

// processMasterKey process master key of Firefox.
// Process the metaBytes and nssA11 with the corresponding cryptographic operations.
func processMasterKey(globalSalt, metaBytes, nssA11, nssA102 []byte) ([]byte, error) {
	metaPBE, err := crypto.NewASN1PBE(metaBytes)
	if err != nil {
		return nil, err
	}

	k, err := metaPBE.Decrypt(globalSalt)
	if err != nil {
		return nil, err
	}

	if !bytes.Contains(k, []byte("password-check")) {
		return nil, errors.New("password-check not found")
	}
	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if !bytes.Equal(nssA102, keyLin) {
		return nil, errors.New("nssA102 not equal keyLin")
	}
	nssPBE, err := crypto.NewASN1PBE(nssA11)
	if err != nil {
		return nil, err
	}
	finallyKey, err := nssPBE.Decrypt(globalSalt)
	if err != nil {
		return nil, err
	}
	if len(finallyKey) < 24 {
		return nil, errors.New("finallyKey length less than 24")
	}
	finallyKey = finallyKey[:24]
	return finallyKey, nil
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
