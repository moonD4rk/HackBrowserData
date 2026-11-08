package firefox

import (
	"database/sql"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const historyQuery = `SELECT url, COALESCE(last_visit_date, 0),
	COALESCE(title, ''), visit_count FROM moz_places`

func extractHistories(path string, out browser.Collector) error {
	return sqlitex.EachRow(path, historyQuery,
		func(rows *sql.Rows) error {
			var url, title string
			var visitCount int
			var lastVisit int64
			if err := rows.Scan(&url, &lastVisit, &title, &visitCount); err != nil {
				return err
			}
			out.Add(&record.History{
				URL:        url,
				Title:      title,
				VisitCount: visitCount,
				LastVisit:  timeconv.UnixMicro(lastVisit),
			})
			return nil
		}, out.Fail, sqlitex.JournalOff)
}
