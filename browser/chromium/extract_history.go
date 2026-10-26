package chromium

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	defaultHistoryQuery = `SELECT url, title, visit_count, last_visit_time FROM urls`
	countHistoryQuery   = `SELECT COUNT(*) FROM urls`
)

func extractHistories(path string) ([]record.History, error) {
	histories, err := sqlitex.QueryRows(path, false, defaultHistoryQuery,
		func(rows *sql.Rows) (record.History, error) {
			var url, title string
			var visitCount int
			var lastVisit int64
			if err := rows.Scan(&url, &title, &visitCount, &lastVisit); err != nil {
				return record.History{}, err
			}
			return record.History{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.Chromium(lastVisit),
			}, nil
		})
	if err != nil {
		return nil, err
	}
	return histories, nil
}

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, false, countHistoryQuery)
}
