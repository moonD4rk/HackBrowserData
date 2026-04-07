package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	defaultDownloadQuery = `SELECT target_path, tab_url, total_bytes, start_time, end_time,
		mime_type FROM downloads`
	countDownloadQuery = `SELECT COUNT(*) FROM downloads`
)

func extractDownloads(path string) ([]types.DownloadEntry, error) {
	downloads, err := sqliteutil.QueryRows(path, false, defaultDownloadQuery,
		func(rows *sql.Rows) (types.DownloadEntry, error) {
			var targetPath, url, mimeType string
			var totalBytes, startTime, endTime int64
			if err := rows.Scan(&targetPath, &url, &totalBytes, &startTime, &endTime, &mimeType); err != nil {
				return types.DownloadEntry{}, err
			}
			return types.DownloadEntry{
				URL:        url,
				TargetPath: targetPath,
				MimeType:   mimeType,
				TotalBytes: totalBytes,
				StartTime:  timeEpoch(startTime),
				EndTime:    timeEpoch(endTime),
			}, nil
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
	return sqliteutil.CountRows(path, false, countDownloadQuery)
}
