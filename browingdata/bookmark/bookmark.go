package bookmark

import (
	"database/sql"
	"os"
	"sort"
	"time"

	// import sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"

	"github.com/moond4rk/HackBrowserData/item"
	"github.com/moond4rk/HackBrowserData/log"
	"github.com/moond4rk/HackBrowserData/utils/fileutil"
	"github.com/moond4rk/HackBrowserData/utils/typeutil"
)

type ChromiumBookmark []bookmark

type bookmark struct {
	ID        int64
	Name      string
	Type      string
	URL       string
	DateAdded time.Time
}

func (c *ChromiumBookmark) Parse(_ []byte) error {
	bookmarks, err := fileutil.ReadFile(item.TempChromiumBookmark)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempChromiumBookmark)
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

const (
	bookmarkID       = "id"
	bookmarkAdded    = "date_added"
	bookmarkURL      = "url"
	bookmarkName     = "name"
	bookmarkType     = "type"
	bookmarkChildren = "children"
)

func getBookmarkChildren(value gjson.Result, w *ChromiumBookmark) (children gjson.Result) {
	nodeType := value.Get(bookmarkType)
	children = value.Get(bookmarkChildren)

	bm := bookmark{
		ID:        value.Get(bookmarkID).Int(),
		Name:      value.Get(bookmarkName).String(),
		URL:       value.Get(bookmarkURL).String(),
		DateAdded: typeutil.TimeEpoch(value.Get(bookmarkAdded).Int()),
	}
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

func (c *ChromiumBookmark) Len() int {
	return len(*c)
}

type FirefoxBookmark []bookmark

const (
	queryFirefoxBookMark = `SELECT id, url, type, dateAdded, title FROM (SELECT * FROM moz_bookmarks INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id)`
	closeJournalMode     = `PRAGMA journal_mode=off`
)

func (f *FirefoxBookmark) Parse(_ []byte) error {
	db, err := sql.Open("sqlite3", item.TempFirefoxBookmark)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempFirefoxBookmark)
	defer db.Close()
	_, err = db.Exec(closeJournalMode)
	if err != nil {
		log.Error(err)
	}
	rows, err := db.Query(queryFirefoxBookMark)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, bt, dateAdded int64
			title, url        string
		)
		if err = rows.Scan(&id, &url, &bt, &dateAdded, &title); err != nil {
			log.Warn(err)
		}
		*f = append(*f, bookmark{
			ID:        id,
			Name:      title,
			Type:      linkType(bt),
			URL:       url,
			DateAdded: typeutil.TimeStamp(dateAdded / 1000000),
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

func (f *FirefoxBookmark) Len() int {
	return len(*f)
}

func linkType(a int64) string {
	switch a {
	case 1:
		return "url"
	default:
		return "folder"
	}
}
