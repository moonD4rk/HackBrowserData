package history

import (
	"database/sql"
	"os"
	"sort"
	"time"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils/typeutil"

	// import sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
)

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

func (c *ChromiumHistory) Parse(masterKey []byte) error {
	historyDB, err := sql.Open("sqlite3", item.TempChromiumHistory)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempChromiumHistory)
	defer historyDB.Close()
	rows, err := historyDB.Query(queryChromiumHistory)
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
			log.Warn(err)
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

func (c *ChromiumHistory) Length() int {
	return len(*c)
}

type FirefoxHistory []history

const (
	queryFirefoxHistory = `SELECT id, url, last_visit_date, title, visit_count FROM moz_places where title not null`
	closeJournalMode    = `PRAGMA journal_mode=off`
)

func (f *FirefoxHistory) Parse(masterKey []byte) error {
	var (
		err         error
		keyDB       *sql.DB
		historyRows *sql.Rows
	)
	keyDB, err = sql.Open("sqlite3", item.TempFirefoxHistory)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempFirefoxHistory)
	defer keyDB.Close()
	_, err = keyDB.Exec(closeJournalMode)
	if err != nil {
		return err
	}
	defer keyDB.Close()
	historyRows, err = keyDB.Query(queryFirefoxHistory)
	if err != nil {
		return err
	}
	defer historyRows.Close()
	for historyRows.Next() {
		var (
			id, visitDate int64
			url, title    string
			visitCount    int
		)
		if err = historyRows.Scan(&id, &url, &visitDate, &title, &visitCount); err != nil {
			log.Warn(err)
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

func (f *FirefoxHistory) Length() int {
	return len(*f)
}
