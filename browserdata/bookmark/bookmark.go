package bookmark

import (
	"database/sql"
	"os"
	"sort"
	"time"

	"github.com/tidwall/gjson"
	_ "modernc.org/sqlite" // import sqlite3 driver

	"github.com/moond4rk/hackbrowserdata/extractor"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

func init() {
	extractor.RegisterExtractor(types.ChromiumBookmark, func() extractor.Extractor {
		return new(ChromiumBookmark)
	})
	extractor.RegisterExtractor(types.FirefoxBookmark, func() extractor.Extractor {
		return new(FirefoxBookmark)
	})
}

type ChromiumBookmark []bookmark

type bookmark struct {
	ID        int64
	Name      string
	Type      string
	URL       string
	DateAdded time.Time
}

func (c *ChromiumBookmark) Extract(_ []byte) error {
	bookmarks, err := fileutil.ReadFile(types.ChromiumBookmark.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.ChromiumBookmark.TempFilename())
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

func (f *FirefoxBookmark) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.FirefoxBookmark.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.FirefoxBookmark.TempFilename())
	defer db.Close()
	_, err = db.Exec(closeJournalMode)
	if err != nil {
		log.Errorf("close journal mode error: %v", err)
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
			log.Errorf("scan bookmark error: %v", err)
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
