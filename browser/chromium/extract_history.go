package chromium

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

const defaultHistoryQuery = `SELECT url, title, visit_count, last_visit_time FROM urls`

func extractHistories(path string) ([]types.HistoryEntry, error) {
	histories, err := sqliteutil.QueryRows(path, false, defaultHistoryQuery,
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
				LastVisit:  typeutil.TimeEpoch(lastVisit),
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
