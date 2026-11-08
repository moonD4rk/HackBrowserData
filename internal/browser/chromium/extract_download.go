package chromium

import (
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const downloadQuery = `SELECT target_path, tab_url, total_bytes, start_time, end_time,
	mime_type FROM downloads`

func extractDownloads(path string, out browser.Collector) error {
	database, err := sqlitex.Open(path)
	if err != nil {
		return err
	}
	defer database.Close()

	rows, err := database.Query(downloadQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var targetPath, url, mimeType string
		var totalBytes, startTime, endTime int64
		if err := rows.Scan(&targetPath, &url, &totalBytes, &startTime, &endTime, &mimeType); err != nil {
			out.Fail(err)
			continue
		}
		out.Add(&record.Download{
			URL:        url,
			TargetPath: targetPath,
			MimeType:   mimeType,
			TotalBytes: totalBytes,
			StartTime:  timeconv.Chromium(startTime),
			EndTime:    timeconv.Chromium(endTime),
		})
	}
	return rows.Err()
}
