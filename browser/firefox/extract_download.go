package firefox

import (
	"database/sql"
	"sort"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	firefoxDownloadQuery = `SELECT place_id, GROUP_CONCAT(content), url, dateAdded
		FROM (SELECT * FROM moz_annos INNER JOIN moz_places ON moz_annos.place_id=moz_places.id)
		t GROUP BY place_id`
	firefoxCountDownloadQuery = `SELECT COUNT(*) FROM
		(SELECT place_id FROM moz_annos
			INNER JOIN moz_places ON moz_annos.place_id=moz_places.id
		GROUP BY place_id)`
)

func extractDownloads(path string) ([]types.DownloadEntry, error) {
	downloads, err := sqliteutil.QueryRows(path, true, firefoxDownloadQuery,
		func(rows *sql.Rows) (types.DownloadEntry, error) {
			var placeID, dateAdded int64
			var content, url string
			if err := rows.Scan(&placeID, &content, &url, &dateAdded); err != nil {
				return types.DownloadEntry{}, err
			}

			entry := types.DownloadEntry{
				URL:       url,
				StartTime: firefoxMicros(dateAdded),
			}

			// Firefox stores download metadata as: "target_path,{json}"
			// Parse the JSON part to extract fileSize and endTime.
			contentList := strings.SplitN(content, ",{", 2)
			if len(contentList) == 2 {
				entry.TargetPath = contentList[0]
				json := "{" + contentList[1]
				entry.TotalBytes = gjson.Get(json, "fileSize").Int()
				entry.EndTime = firefoxMillis(gjson.Get(json, "endTime").Int())
			} else {
				entry.TargetPath = content
			}

			return entry, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(downloads, func(i, j int) bool {
		return downloads[i].StartTime.After(downloads[j].StartTime)
	})
	return downloads, nil
}

func countDownloads(path string) (int, error) {
	return sqliteutil.CountRows(path, true, firefoxCountDownloadQuery)
}
