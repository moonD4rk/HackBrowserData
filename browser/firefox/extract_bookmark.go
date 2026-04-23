package firefox

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	firefoxBookmarkQuery = `SELECT id, url, type, dateAdded, COALESCE(title, '')
		FROM (SELECT * FROM moz_bookmarks INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id)`
	firefoxCountBookmarkQuery = `SELECT COUNT(*) FROM moz_bookmarks
		INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id`
)

func extractBookmarks(path string) ([]types.BookmarkEntry, error) {
	bookmarks, err := sqliteutil.QueryRows(path, true, firefoxBookmarkQuery,
		func(rows *sql.Rows) (types.BookmarkEntry, error) {
			var id, dateAdded int64
			var url, title string
			var bt int64
			if err := rows.Scan(&id, &url, &bt, &dateAdded, &title); err != nil {
				return types.BookmarkEntry{}, err
			}
			return types.BookmarkEntry{
				Name:      title,
				URL:       url,
				Folder:    bookmarkType(bt),
				CreatedAt: firefoxMicros(dateAdded),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(bookmarks, func(i, j int) bool {
		return bookmarks[i].CreatedAt.After(bookmarks[j].CreatedAt)
	})
	return bookmarks, nil
}

func bookmarkType(bt int64) string {
	switch bt {
	case 1:
		return "url"
	default:
		return "folder"
	}
}

func countBookmarks(path string) (int, error) {
	return sqliteutil.CountRows(path, true, firefoxCountBookmarkQuery)
}
