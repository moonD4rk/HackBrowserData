package chromium

import (
	"bytes"
	"fmt"
	"os"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/moond4rk/hackbrowserdata/types"
)

func extractLocalStorage(path string) ([]types.StorageEntry, error) {
	return extractLevelDB(path, []byte("\x00"))
}

func extractSessionStorage(path string) ([]types.StorageEntry, error) {
	return extractLevelDB(path, []byte("-"))
}

// extractLevelDB iterates over all entries in a LevelDB directory,
// splitting each key by the separator into (url, name).
func extractLevelDB(path string, separator []byte) ([]types.StorageEntry, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("leveldb path not found: %s", path)
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var entries []types.StorageEntry
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		url, name := parseStorageKey(iter.Key(), separator)
		if url == "" {
			continue
		}
		entries = append(entries, types.StorageEntry{
			URL:   url,
			Key:   name,
			Value: string(iter.Value()),
		})
	}
	return entries, iter.Error()
}

// parseStorageKey splits a LevelDB key into (url, name) by the given separator.
func parseStorageKey(key, separator []byte) (url, name string) {
	parts := bytes.SplitN(key, separator, 2)
	if len(parts) != 2 {
		return "", ""
	}
	return string(parts[0]), string(parts[1])
}
