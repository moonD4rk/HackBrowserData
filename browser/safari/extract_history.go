package safari

import (
	"database/sql"
	"sort"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	// safariHistoryQuery joins history_items with history_visits to get
	// the latest visit per URL. GROUP BY deduplicates multiple visits.
	safariHistoryQuery = `SELECT hi.url, COALESCE(hv.title, ''), hi.visit_count,
		COALESCE(MAX(hv.visit_time), 0)
		FROM history_items hi
		LEFT JOIN history_visits hv ON hi.id = hv.history_item
		GROUP BY hi.id`

	safariCountHistoryQuery = `SELECT COUNT(*) FROM history_items`
)

func extractHistories(path string) ([]types.HistoryEntry, error) {
	histories, err := sqliteutil.QueryRows(path, true, safariHistoryQuery,
		func(rows *sql.Rows) (types.HistoryEntry, error) {
			var (
				url, title string
				visitCount int
				visitTime  float64
			)
			if err := rows.Scan(&url, &title, &visitCount, &visitTime); err != nil {
				return types.HistoryEntry{}, err
			}
			return types.HistoryEntry{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  coredataTimestamp(visitTime),
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
	return sqliteutil.CountRows(path, true, safariCountHistoryQuery)
}
