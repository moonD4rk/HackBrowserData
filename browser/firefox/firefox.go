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

	"github.com/moond4rk/hackbrowserdata/browserdata"
	"github.com/moond4rk/hackbrowserdata/crypto"
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

	nssA11, nssA102, err := queryNssPrivate(keyDB)
	if err != nil {
		return nil, fmt.Errorf("query NSS private error: %w", err)
	}

	return processMasterKey(metaItem1, metaItem2, nssA11, nssA102)
}

func queryMetaData(db *sql.DB) ([]byte, []byte, error) {
	const query = `SELECT item1, item2 FROM metaData WHERE id = 'password'`
	var metaItem1, metaItem2 []byte
	if err := db.QueryRow(query).Scan(&metaItem1, &metaItem2); err != nil {
		return nil, nil, err
	}
	return metaItem1, metaItem2, nil
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
func processMasterKey(metaItem1, metaItem2, nssA11, nssA102 []byte) ([]byte, error) {
	metaPBE, err := crypto.NewASN1PBE(metaItem2)
	if err != nil {
		return nil, fmt.Errorf("error creating ASN1PBE from metaItem2: %w", err)
	}

	flag, err := metaPBE.Decrypt(metaItem1)
	if err != nil {
		return nil, fmt.Errorf("error decrypting master key: %w", err)
	}
	const passwordCheck = "password-check"

	if !bytes.Contains(flag, []byte(passwordCheck)) {
		return nil, errors.New("flag verification failed: password-check not found")
	}

	keyLin := []byte{248, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if !bytes.Equal(nssA102, keyLin) {
		return nil, errors.New("master key verification failed: nssA102 not equal to expected value")
	}

	nssA11PBE, err := crypto.NewASN1PBE(nssA11)
	if err != nil {
		return nil, fmt.Errorf("error creating ASN1PBE from nssA11: %w", err)
	}

	finallyKey, err := nssA11PBE.Decrypt(metaItem1)
	if err != nil {
		return nil, fmt.Errorf("error decrypting final key: %w", err)
	}
	if len(finallyKey) < 24 {
		return nil, errors.New("length of final key is less than 24 bytes")
	}
	return finallyKey[:24], nil
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
