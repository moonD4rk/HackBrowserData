package chromium

import (
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

const historyQuery = `SELECT url, title, visit_count, last_visit_time FROM urls`

func extractHistories(path string, out browser.Collector) error {
	database, err := sqlitex.Open(path)
	if err != nil {
		return err
	}
	defer database.Close()

	rows, err := database.Query(historyQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var url, title string
		var visitCount int
		var lastVisit int64
		if err := rows.Scan(&url, &title, &visitCount, &lastVisit); err != nil {
			out.Fail(err)
			continue
		}
		out.Add(&record.History{
			URL:        url,
			Title:      title,
			VisitCount: visitCount,
			LastVisit:  timeconv.Chromium(lastVisit),
		})
	}
	return rows.Err()
}
