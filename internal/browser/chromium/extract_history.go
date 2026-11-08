package chromium

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const historyQuery = `SELECT url, title, visit_count, last_visit_time FROM urls`

func extractHistories(path string, out browser.Collector) error {
	return sqlitex.EachRow(path, historyQuery,
		func(rows *sql.Rows) error {
			var url, title string
			var visitCount int
			var lastVisit int64
			if err := rows.Scan(&url, &title, &visitCount, &lastVisit); err != nil {
				return err
			}
			out.Add(&record.History{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.Chromium(lastVisit),
			})
			return nil
		}, out.Fail)
}
