package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
	"github.com/moond4rk/hackbrowserdata/types"
)

const (
	defaultHistoryQuery = `SELECT url, title, visit_count, last_visit_time FROM urls`
	countHistoryQuery   = `SELECT COUNT(*) FROM urls`
)

func extractHistories(path string) ([]types.HistoryEntry, error) {
	histories, err := sqlitex.QueryRows(path, false, defaultHistoryQuery,
		func(rows *sql.Rows) (types.HistoryEntry, error) {
			var url, title string
			var visitCount int
			var lastVisit int64
			if err := rows.Scan(&url, &title, &visitCount, &lastVisit); err != nil {
				return types.HistoryEntry{}, err
			}
			return types.HistoryEntry{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.Chromium(lastVisit),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(histories, func(i, j int) bool {
		return histories[i].VisitCount > histories[j].VisitCount
	})
	return histories, nil
}

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, false, countHistoryQuery)
}
