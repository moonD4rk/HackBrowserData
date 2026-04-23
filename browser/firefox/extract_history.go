package firefox

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	firefoxHistoryQuery = `SELECT url, COALESCE(last_visit_date, 0),
		COALESCE(title, ''), visit_count FROM moz_places`
	firefoxCountHistoryQuery = `SELECT COUNT(*) FROM moz_places`
)

func extractHistories(path string) ([]types.HistoryEntry, error) {
	histories, err := sqliteutil.QueryRows(path, true, firefoxHistoryQuery,
		func(rows *sql.Rows) (types.HistoryEntry, error) {
			var url, title string
			var visitCount int
			var lastVisit int64
			if err := rows.Scan(&url, &lastVisit, &title, &visitCount); err != nil {
				return types.HistoryEntry{}, err
			}
			return types.HistoryEntry{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  firefoxMicros(lastVisit),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(histories, func(i, j int) bool {
		return histories[i].VisitCount < histories[j].VisitCount
	})
	return histories, nil
}

func countHistories(path string) (int, error) {
	return sqliteutil.CountRows(path, true, firefoxCountHistoryQuery)
}
