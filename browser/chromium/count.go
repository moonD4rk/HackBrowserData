package chromium

import (
	"bytes"
	"os"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

// list --detail keeps today's counting queries until the switch-over, when it counts what the extractors emit.
const (
	countLoginQuery            = `SELECT COUNT(*) FROM logins`
	countCookieQuery           = `SELECT COUNT(*) FROM cookies`
	countHistoryQuery          = `SELECT COUNT(*) FROM urls`
	countDownloadQuery         = `SELECT COUNT(*) FROM downloads`
	countCreditCardQuery       = `SELECT COUNT(*) FROM credit_cards`
	yandexCreditCardCountQuery = `SELECT COUNT(*) FROM records`
)

// defaultExtensionKeys are the JSON paths tried for standard Chromium browsers.
var defaultExtensionKeys = []string{
	"extensions.settings",
	"settings.extensions",
	"settings.settings",
}

func countPasswords(path string) (int, error) {
	return sqlitex.CountRows(path, false, countLoginQuery)
}

func countCookies(path string) (int, error) {
	return sqlitex.CountRows(path, false, countCookieQuery)
}

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, false, countHistoryQuery)
}

func countDownloads(path string) (int, error) {
	return sqlitex.CountRows(path, false, countDownloadQuery)
}

func countBookmarks(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var count int
	roots := gjson.GetBytes(data, "roots")
	roots.ForEach(func(_, value gjson.Result) bool {
		count += walkCountBookmarks(value)
		return true
	})
	return count, nil
}

// walkCountBookmarks recursively counts URL nodes in the bookmark tree.
func walkCountBookmarks(node gjson.Result) int {
	count := 0
	if node.Get("type").String() == "url" {
		count++
	}
	children := node.Get("children")
	if children.Exists() && children.IsArray() {
		for _, child := range children.Array() {
			count += walkCountBookmarks(child)
		}
	}
	return count
}

func countCreditCards(path string) (int, error) {
	return sqlitex.CountRows(path, false, countCreditCardQuery)
}

func countYandexCreditCards(path string) (int, error) {
	return sqlitex.CountRows(path, false, yandexCreditCardCountQuery)
}

func countExtensions(path string) (int, error) {
	return countExtensionsWithKeys(path, defaultExtensionKeys)
}

func countOperaExtensions(path string) (int, error) {
	return countExtensionsWithKeys(path, []string{"extensions.opsettings"})
}

// countExtensionsWithKeys counts non-system extensions without building
// full ExtensionEntry structs. Mirrors the filtering logic in extractExtensionsWithKeys.
func countExtensionsWithKeys(path string, keys []string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	var settings gjson.Result
	for _, key := range keys {
		settings = gjson.GetBytes(data, key)
		if settings.Exists() {
			break
		}
	}
	if !settings.Exists() {
		return 0, nil
	}

	var count int
	settings.ForEach(func(_, ext gjson.Result) bool {
		location := ext.Get("location").Int()
		if location == 5 || location == 10 {
			return true
		}
		if !ext.Get("manifest").Exists() {
			return true
		}
		count++
		return true
	})
	return count, nil
}

func countLocalStorage(path string) (int, error) {
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var count int
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		if isLocalStorageEntry(iter.Key()) {
			count++
		}
	}
	return count, iter.Error()
}

func countSessionStorage(path string) (int, error) {
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var count int
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	mapPrefix := []byte("map-")
	for iter.Next() {
		if bytes.HasPrefix(iter.Key(), mapPrefix) {
			if sep := bytes.IndexByte(iter.Key()[len(mapPrefix):], '-'); sep >= 0 {
				count++
			}
		}
	}
	return count, iter.Error()
}

// isLocalStorageEntry keeps the keys the localStorage extractor emits: data and META entries, never VERSION.
func isLocalStorageEntry(key []byte) bool {
	switch {
	case bytes.Equal(key, []byte("VERSION")):
		return false
	case bytes.HasPrefix(key, []byte("METAACCESS:")), bytes.HasPrefix(key, []byte("META:")):
		return true
	default:
		return len(key) > 0 && key[0] == '_'
	}
}
