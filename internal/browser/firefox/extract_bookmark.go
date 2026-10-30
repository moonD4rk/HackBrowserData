package firefox

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

// Folders and separators have no place, so the place join keeps only bookmarks; the parent join names their folder.
const firefoxBookmarkQuery = `SELECT b.id, p.url, b.type, b.dateAdded, COALESCE(b.title, ''), COALESCE(f.title, '')
	FROM moz_bookmarks b
	INNER JOIN moz_places p ON b.fk = p.id
	LEFT JOIN moz_bookmarks f ON b.parent = f.id`

func extractBookmarks(path string, out browser.Collector) error {
	bookmarks, err := sqlitex.QueryRows(path, true, firefoxBookmarkQuery,
		func(rows *sql.Rows) (record.Bookmark, error) {
			var id, dateAdded int64
			var url, title, folder string
			var bt int64
			if err := rows.Scan(&id, &url, &bt, &dateAdded, &title, &folder); err != nil {
				return record.Bookmark{}, err
			}
			return record.Bookmark{
				ID:        id,
				Name:      title,
				Type:      bookmarkType(bt),
				URL:       url,
				Folder:    folder,
				CreatedAt: timeconv.UnixMicro(dateAdded),
			}, nil
		})
	if err != nil {
		return err
	}
	emit(out, bookmarks)
	return nil
}

func bookmarkType(bt int64) string {
	switch bt {
	case 1:
		return "url"
	default:
		return "folder"
	}
}
