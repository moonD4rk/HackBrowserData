package firefox

import (
	"database/sql"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
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

func extractDownloads(path string) ([]record.Download, error) {
	downloads, err := sqlitex.QueryRows(path, true, firefoxDownloadQuery,
		func(rows *sql.Rows) (record.Download, error) {
			var placeID, dateAdded int64
			var content, url string
			if err := rows.Scan(&placeID, &content, &url, &dateAdded); err != nil {
				return record.Download{}, err
			}

			entry := record.Download{
				URL:       url,
				StartTime: timeconv.UnixMicro(dateAdded),
			}

			// Firefox stores download metadata as: "target_path,{json}"
			// Parse the JSON part to extract fileSize and endTime.
			contentList := strings.SplitN(content, ",{", 2)
			if len(contentList) == 2 {
				entry.TargetPath = contentList[0]
				json := "{" + contentList[1]
				entry.TotalBytes = gjson.Get(json, "fileSize").Int()
				entry.EndTime = timeconv.UnixMilli(gjson.Get(json, "endTime").Int())
			} else {
				entry.TargetPath = content
			}

			return entry, nil
		})
	if err != nil {
		return nil, err
	}
	return downloads, nil
}

func countDownloads(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountDownloadQuery)
}
