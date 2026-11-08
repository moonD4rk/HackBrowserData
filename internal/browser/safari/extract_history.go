package safari

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const (
	// historyQuery joins each item's latest visit, so title and visit_time come from the same history_visits row.
	historyQuery = `SELECT hi.url, COALESCE(hv.title, ''), hi.visit_count,
		COALESCE(hv.visit_time, 0)
		FROM history_items hi
		LEFT JOIN history_visits hv ON hv.id = (
			SELECT hv2.id FROM history_visits hv2
			WHERE hv2.history_item = hi.id
			ORDER BY hv2.visit_time DESC LIMIT 1
		)`
)

func extractHistories(path string, out browser.Collector) error {
	return sqlitex.EachRow(path, historyQuery,
		func(rows *sql.Rows) error {
			var (
				url, title string
				visitCount int
				visitTime  float64
			)
			if err := rows.Scan(&url, &title, &visitCount, &visitTime); err != nil {
				return err
			}
			out.Add(&record.History{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.CoreData(visitTime),
			})
			return nil
		}, out.Fail, sqlitex.JournalOff)
}
