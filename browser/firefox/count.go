package firefox

import (
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

// list --detail keeps today's counting queries until the switch-over, when it counts what the extractors emit.
const (
	firefoxCountCookieQuery   = `SELECT COUNT(*) FROM moz_cookies`
	firefoxCountHistoryQuery  = `SELECT COUNT(*) FROM moz_places`
	firefoxCountDownloadQuery = `SELECT COUNT(*) FROM
		(SELECT place_id FROM moz_annos
			INNER JOIN moz_places ON moz_annos.place_id=moz_places.id
		GROUP BY place_id)`
	firefoxCountBookmarkQuery = `SELECT COUNT(*) FROM moz_bookmarks
		INNER JOIN moz_places ON moz_bookmarks.fk=moz_places.id`
	firefoxCountLocalStorageQuery = `SELECT COUNT(*) FROM webappsstore2`
)

func countPasswords(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return len(gjson.GetBytes(data, "logins").Array()), nil
}

func countCookies(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountCookieQuery)
}

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountHistoryQuery)
}

func countDownloads(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountDownloadQuery)
}

func countBookmarks(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountBookmarkQuery)
}

func countExtensions(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	var count int
	for _, v := range gjson.GetBytes(data, "addons").Array() {
		if v.Get("location").String() == "app-profile" {
			count++
		}
	}
	return count, nil
}

func countLocalStorage(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountLocalStorageQuery)
}
