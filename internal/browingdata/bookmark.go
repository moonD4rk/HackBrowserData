package browingdata

import (
	"database/sql"
	"os"
	"sort"
	"time"

	"github.com/tidwall/gjson"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils"
	"hack-browser-data/internal/utils/fileutil"

	_ "github.com/mattn/go-sqlite3"
)

type ChromiumBookmark []bookmark

type bookmark struct {
	ID        int64
	Name      string
	Type      string
	URL       string
	DateAdded time.Time
}

func (c *ChromiumBookmark) Parse(masterKey []byte) error {
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
	// TODO: refactor with go generics
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
	keyDB, err = sql.Open("sqlite3", item.TempFirefoxBookmark)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempFirefoxBookmark)
	defer keyDB.Close()
	_, err = keyDB.Exec(closeJournalMode)
	if err != nil {
		log.Error(err)
	}
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
			log.Warn(err)
		}
		*f = append(*f, bookmark{
			ID:        id,
			Name:      title,
			Type:      utils.BookmarkType(bType),
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
