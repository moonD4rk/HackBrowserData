package firefox

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

// bookmarkQuery keeps only bookmarks, as folders and separators have no place; the parent join names the folder.
const bookmarkQuery = `SELECT b.id, p.url, b.type, b.dateAdded, COALESCE(b.title, ''), COALESCE(f.title, '')
	FROM moz_bookmarks b
	INNER JOIN moz_places p ON b.fk = p.id
	LEFT JOIN moz_bookmarks f ON b.parent = f.id`

func extractBookmarks(path string, out browser.Collector) error {
	bookmarks, err := sqlitex.QueryRows(path, true, bookmarkQuery,
		func(rows *sql.Rows) (record.Bookmark, error) {
			var id, dateAdded int64
			var url, title, folder string
			var typeCode int64
			if err := rows.Scan(&id, &url, &typeCode, &dateAdded, &title, &folder); err != nil {
				return record.Bookmark{}, err
			}
			return record.Bookmark{
				ID:        id,
				Name:      title,
				Type:      bookmarkType(typeCode),
				URL:       url,
				Folder:    folder,
				CreatedAt: timeconv.UnixMicro(dateAdded),
			}, nil
		})
	if err != nil {
		return err
	}
	browser.AddAll(out, bookmarks)
	return nil
}

func bookmarkType(typeCode int64) string {
	switch typeCode {
	case 1:
		return "url"
	default:
		return "folder"
	}
}
