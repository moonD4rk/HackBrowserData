package data

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/tidwall/gjson"

	"hack-browser-data/internal/browser/item"
	"hack-browser-data/internal/utils"

	_ "github.com/mattn/go-sqlite3"
)

type ChromiumDownload []download

func (c *ChromiumDownload) Parse(masterKey []byte) error {
	historyDB, err := sql.Open("sqlite3", item.ChromiumDownloadFilename)
	if err != nil {
		return err
	}
	defer historyDB.Close()
	rows, err := historyDB.Query(queryChromiumDownload)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			targetPath, tabUrl, mimeType   string
			totalBytes, startTime, endTime int64
		)
		if err := rows.Scan(&targetPath, &tabUrl, &totalBytes, &startTime, &endTime, &mimeType); err != nil {
			fmt.Println(err)
		}
		data := download{
			TargetPath: targetPath,
			Url:        tabUrl,
			TotalBytes: totalBytes,
			StartTime:  utils.TimeEpochFormat(startTime),
			EndTime:    utils.TimeEpochFormat(endTime),
			MimeType:   mimeType,
		}
		*c = append(*c, data)
	}
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].TotalBytes > (*c)[j].TotalBytes
	})
	return nil
}

func (c *ChromiumDownload) Name() string {
	return "download"
}

type FirefoxDownload []download

func (f *FirefoxDownload) Parse(masterKey []byte) error {
	var (
		err          error
		keyDB        *sql.DB
		downloadRows *sql.Rows
	)
	keyDB, err = sql.Open("sqlite3", item.FirefoxDownloadFilename)
	if err != nil {
		return err
	}
	_, err = keyDB.Exec(closeJournalMode)
	if err != nil {
		return err
	}
	defer keyDB.Close()
	downloadRows, err = keyDB.Query(queryFirefoxDownload)
	if err != nil {
		return err
	}
	defer downloadRows.Close()
	for downloadRows.Next() {
		var (
			content, url       string
			placeID, dateAdded int64
		)
		if err = downloadRows.Scan(&placeID, &content, &url, &dateAdded); err != nil {
			fmt.Println(err)
		}
		contentList := strings.Split(content, ",{")
		if len(contentList) > 1 {
			path := contentList[0]
			json := "{" + contentList[1]
			endTime := gjson.Get(json, "endTime")
			fileSize := gjson.Get(json, "fileSize")
			*f = append(*f, download{
				TargetPath: path,
				Url:        url,
				TotalBytes: fileSize.Int(),
				StartTime:  utils.TimeStampFormat(dateAdded / 1000000),
				EndTime:    utils.TimeStampFormat(endTime.Int() / 1000),
			})
		}
	}
	sort.Slice(*f, func(i, j int) bool {
		return (*f)[i].TotalBytes < (*f)[j].TotalBytes
	})
	return nil
}

func (f *FirefoxDownload) Name() string {
	return "download"
}
