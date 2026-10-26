package firefox

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	firefoxHistoryQuery = `SELECT url, COALESCE(last_visit_date, 0),
		COALESCE(title, ''), visit_count FROM moz_places`
	firefoxCountHistoryQuery = `SELECT COUNT(*) FROM moz_places`
)

func extractHistories(path string) ([]record.History, error) {
	histories, err := sqlitex.QueryRows(path, true, firefoxHistoryQuery,
		func(rows *sql.Rows) (record.History, error) {
			var url, title string
			var visitCount int
			var lastVisit int64
			if err := rows.Scan(&url, &lastVisit, &title, &visitCount); err != nil {
				return record.History{}, err
			}
			return record.History{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.UnixMicro(lastVisit),
			}, nil
		})
	if err != nil {
		return nil, err
	}
	return histories, nil
}

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, true, firefoxCountHistoryQuery)
}
