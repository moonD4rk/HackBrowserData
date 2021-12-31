package data

import (
	"database/sql"
	"fmt"
	"sort"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/utils"
)

type ChromiumHistory []history

func (c *ChromiumHistory) Parse(masterKey []byte) error {
	historyDB, err := sql.Open("sqlite3", consts.ChromiumHistoryFilename)
	if err != nil {
		return err
	}
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
		// TODO: handle rows error
		if err := rows.Scan(&url, &title, &visitCount, &lastVisitTime); err != nil {
			fmt.Println(err)
		}
		data := history{
			Url:           url,
			Title:         title,
			VisitCount:    visitCount,
			LastVisitTime: utils.TimeEpochFormat(lastVisitTime),
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
