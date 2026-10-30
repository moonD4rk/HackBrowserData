package chromium

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const defaultDownloadQuery = `SELECT target_path, tab_url, total_bytes, start_time, end_time,
	mime_type FROM downloads`

func extractDownloads(path string, out browser.Collector) error {
	downloads, err := sqlitex.QueryRows(path, false, defaultDownloadQuery,
		func(rows *sql.Rows) (record.Download, error) {
			var targetPath, url, mimeType string
			var totalBytes, startTime, endTime int64
			if err := rows.Scan(&targetPath, &url, &totalBytes, &startTime, &endTime, &mimeType); err != nil {
				return record.Download{}, err
			}
			return record.Download{
				URL:        url,
				TargetPath: targetPath,
				MimeType:   mimeType,
				TotalBytes: totalBytes,
				StartTime:  timeconv.Chromium(startTime),
				EndTime:    timeconv.Chromium(endTime),
			}, nil
		})
	if err != nil {
		return err
	}
	emit(out, downloads)
	return nil
}
