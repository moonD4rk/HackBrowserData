package data

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/tidwall/gjson"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/utils"
)

type ChromiumBookmark []bookmark

func (c *ChromiumBookmark) Parse(masterKey []byte) error {
	bookmarks, err := utils.ReadFile(consts.ChromiumBookmarkFilename)
	if err != nil {
		return err
	}
	r := gjson.Parse(bookmarks)
	if r.Exists() {
		roots := r.Get("roots")
		roots.ForEach(func(key, value gjson.Result) bool {
			getBookmarkChildren(value, c)
			return true
		})
	}
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].DateAdded.After((*c)[j].DateAdded)
	})
	return nil
}

func getBookmarkChildren(value gjson.Result, w *ChromiumBookmark) (children gjson.Result) {
	const (
		bookmarkID       = "id"
		bookmarkAdded    = "date_added"
		bookmarkUrl      = "url"
		bookmarkName     = "name"
		bookmarkType     = "type"
		bookmarkChildren = "children"
	)
	nodeType := value.Get(bookmarkType)
	bm := bookmark{
		ID:        value.Get(bookmarkID).Int(),
		Name:      value.Get(bookmarkName).String(),
		URL:       value.Get(bookmarkUrl).String(),
		DateAdded: utils.TimeEpochFormat(value.Get(bookmarkAdded).Int()),
	}
	children = value.Get(bookmarkChildren)
	if nodeType.Exists() {
		bm.Type = nodeType.String()
		*w = append(*w, bm)
		if children.Exists() && children.IsArray() {
			for _, v := range children.Array() {
				children = getBookmarkChildren(v, w)
			}
		}
	}
	return children
}

func (c *ChromiumBookmark) Name() string {
	return "bookmark"
}

type FirefoxBookmark []bookmark

func (f *FirefoxBookmark) Parse(masterKey []byte) error {
	var (
		err          error
		keyDB        *sql.DB
		bookmarkRows *sql.Rows
	)
	keyDB, err = sql.Open("sqlite3", consts.FirefoxBookmarkFilename)
	if err != nil {
		return err
	}
	_, err = keyDB.Exec(closeJournalMode)
	defer keyDB.Close()

	bookmarkRows, err = keyDB.Query(queryFirefoxBookMark)
	if err != nil {
		return err
	}
	defer bookmarkRows.Close()
	for bookmarkRows.Next() {
		var (
			id, bType, dateAdded int64
			title, url           string
		)
		if err = bookmarkRows.Scan(&id, &url, &bType, &dateAdded, &title); err != nil {
			fmt.Println(err)
		}
		*f = append(*f, bookmark{
			ID:        id,
			Name:      title,
			Type:      utils.BookMarkType(bType),
			URL:       url,
			DateAdded: utils.TimeStampFormat(dateAdded / 1000000),
		})
	}
	sort.Slice(*f, func(i, j int) bool {
		return (*f)[i].DateAdded.After((*f)[j].DateAdded)
	})
	return nil
}

func (f *FirefoxBookmark) Name() string {
	return "bookmark"
}
