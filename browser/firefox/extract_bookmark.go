package firefox

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	firefoxBookmarkQuery = `SELECT id, url, type, dateAdded, COALESCE(title, '')
		FROM (SELECT * FROM moz_bookmarks INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id)`
	firefoxCountBookmarkQuery = `SELECT COUNT(*) FROM moz_bookmarks
		INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id`
)

func extractBookmarks(path string) ([]record.Bookmark, error) {
	bookmarks, err := sqlitex.QueryRows(path, true, firefoxBookmarkQuery,
		func(rows *sql.Rows) (record.Bookmark, error) {
			var id, dateAdded int64
			var url, title string
			var bt int64
			if err := rows.Scan(&id, &url, &bt, &dateAdded, &title); err != nil {
				return record.Bookmark{}, err
			}
			return record.Bookmark{
				Name:      title,
				URL:       url,
				Folder:    bookmarkType(bt),
				CreatedAt: timeconv.UnixMicro(dateAdded),
			}, nil
		})
	if err != nil {
		return nil, err
	}
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
	return sqlitex.CountRows(path, true, firefoxCountBookmarkQuery)
}
