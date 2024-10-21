package history

import (
	"database/sql"
	"os"
	"sort"
	"time"

	// import sqlite3 driver
	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/extractor"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

func init() {
	extractor.RegisterExtractor(types.ChromiumHistory, func() extractor.Extractor {
		return new(ChromiumHistory)
	})
	extractor.RegisterExtractor(types.FirefoxHistory, func() extractor.Extractor {
		return new(FirefoxHistory)
	})
}

type ChromiumHistory []history

type history struct {
	Title         string
	URL           string
	VisitCount    int
	LastVisitTime time.Time
}

const (
	queryChromiumHistory = `SELECT url, title, visit_count, last_visit_time FROM urls`
)

func (c *ChromiumHistory) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.ChromiumHistory.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.ChromiumHistory.TempFilename())
	defer db.Close()

	rows, err := db.Query(queryChromiumHistory)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			url, title    string
			visitCount    int
			lastVisitTime int64
		)
		if err := rows.Scan(&url, &title, &visitCount, &lastVisitTime); err != nil {
			log.Warnf("scan chromium history error: %v", err)
		}
		data := history{
			URL:           url,
			Title:         title,
			VisitCount:    visitCount,
			LastVisitTime: typeutil.TimeEpoch(lastVisitTime),
		}
		*c = append(*c, data)
	}
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].VisitCount > (*c)[j].VisitCount
	})
	return nil
}

func (c *ChromiumHistory) Name() string {
	return "history"
}

func (c *ChromiumHistory) Len() int {
	return len(*c)
}

type FirefoxHistory []history

const (
	queryFirefoxHistory = `SELECT id, url, COALESCE(last_visit_date, 0), COALESCE(title, ''), visit_count FROM moz_places`
	closeJournalMode    = `PRAGMA journal_mode=off`
)

func (f *FirefoxHistory) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.FirefoxHistory.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.FirefoxHistory.TempFilename())
	defer db.Close()

	_, err = db.Exec(closeJournalMode)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(queryFirefoxHistory)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, visitDate int64
			url, title    string
			visitCount    int
		)
		if err = rows.Scan(&id, &url, &visitDate, &title, &visitCount); err != nil {
			log.Errorf("scan firefox history error: %v", err)
		}
		*f = append(*f, history{
			Title:         title,
			URL:           url,
			VisitCount:    visitCount,
			LastVisitTime: typeutil.TimeStamp(visitDate / 1000000),
		})
	}
	sort.Slice(*f, func(i, j int) bool {
		return (*f)[i].VisitCount < (*f)[j].VisitCount
	})
	return nil
}

func (f *FirefoxHistory) Name() string {
	return "history"
}

func (f *FirefoxHistory) Len() int {
	return len(*f)
}
