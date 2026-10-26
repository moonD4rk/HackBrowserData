package safari

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	// safariHistoryQuery joins each history item to its latest visit so
	// title and visit_time come from the same history_visits row.
	safariHistoryQuery = `SELECT hi.url, COALESCE(hv.title, ''), hi.visit_count,
		COALESCE(hv.visit_time, 0)
		FROM history_items hi
		LEFT JOIN history_visits hv ON hv.id = (
			SELECT hv2.id FROM history_visits hv2
			WHERE hv2.history_item = hi.id
			ORDER BY hv2.visit_time DESC LIMIT 1
		)`

	safariCountHistoryQuery = `SELECT COUNT(*) FROM history_items`
)

func extractHistories(path string) ([]record.History, error) {
	histories, err := sqlitex.QueryRows(path, true, safariHistoryQuery,
		func(rows *sql.Rows) (record.History, error) {
			var (
				url, title string
				visitCount int
				visitTime  float64
			)
			if err := rows.Scan(&url, &title, &visitCount, &visitTime); err != nil {
				return record.History{}, err
			}
			return record.History{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.CoreData(visitTime),
			}, nil
		})
	if err != nil {
		return nil, err
	}
	return histories, nil
}

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, true, safariCountHistoryQuery)
}
