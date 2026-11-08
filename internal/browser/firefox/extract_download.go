package firefox

import (
	"database/sql"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const downloadQuery = `SELECT place_id, GROUP_CONCAT(content), url, dateAdded
	FROM (SELECT * FROM moz_annos INNER JOIN moz_places ON moz_annos.place_id=moz_places.id)
	t GROUP BY place_id`

func extractDownloads(path string, out browser.Collector) error {
	return sqlitex.EachRow(path, downloadQuery,
		func(rows *sql.Rows) error {
			var placeID, dateAdded int64
			var content, url string
			if err := rows.Scan(&placeID, &content, &url, &dateAdded); err != nil {
				return err
			}

			entry := record.Download{
				URL:       url,
				StartTime: timeconv.UnixMicro(dateAdded),
			}

			// Firefox stores the content as "target_path,{json}"; the JSON holds fileSize and endTime.
			contentList := strings.SplitN(content, ",{", 2)
			if len(contentList) == 2 {
				entry.TargetPath = contentList[0]
				json := "{" + contentList[1]
				entry.TotalBytes = gjson.Get(json, "fileSize").Int()
				entry.EndTime = timeconv.UnixMilli(gjson.Get(json, "endTime").Int())
			} else {
				entry.TargetPath = content
			}

			out.Add(&entry)
			return nil
		}, out.Fail, sqlitex.JournalOff)
}
