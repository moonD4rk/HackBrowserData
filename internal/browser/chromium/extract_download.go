package chromium

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const downloadQuery = `SELECT target_path, tab_url, total_bytes, start_time, end_time,
	mime_type FROM downloads`

func extractDownloads(path string, out browser.Collector) error {
	return sqlitex.EachRow(path, downloadQuery,
		func(rows *sql.Rows) error {
			var targetPath, url, mimeType string
			var totalBytes, startTime, endTime int64
			if err := rows.Scan(&targetPath, &url, &totalBytes, &startTime, &endTime, &mimeType); err != nil {
				return err
			}
			out.Add(&record.Download{
				URL:        url,
				TargetPath: targetPath,
				MimeType:   mimeType,
				TotalBytes: totalBytes,
				StartTime:  timeconv.Chromium(startTime),
				EndTime:    timeconv.Chromium(endTime),
			})
			return nil
		}, out.Fail)
}
