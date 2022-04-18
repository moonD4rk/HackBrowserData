package download

import (
	"database/sql"
	"os"
	"sort"
	"strings"
	"time"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/utils/typeutil"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tidwall/gjson"
)

type ChromiumDownload []download

type download struct {
	TargetPath string
	Url        string
	TotalBytes int64
	StartTime  time.Time
	EndTime    time.Time
	MimeType   string
}

const (
	queryChromiumDownload = `SELECT target_path, tab_url, total_bytes, start_time, end_time, mime_type FROM downloads`
)

func (c *ChromiumDownload) Parse(masterKey []byte) error {
	historyDB, err := sql.Open("sqlite3", item.TempChromiumDownload)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempChromiumDownload)
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
			log.Warn(err)
		}
		data := download{
			TargetPath: targetPath,
			Url:        tabUrl,
			TotalBytes: totalBytes,
			StartTime:  typeutil.TimeEpoch(startTime),
			EndTime:    typeutil.TimeEpoch(endTime),
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

const (
	queryFirefoxDownload = `SELECT place_id, GROUP_CONCAT(content), url, dateAdded FROM (SELECT * FROM moz_annos INNER JOIN moz_places ON moz_annos.place_id=moz_places.id) t GROUP BY place_id`
	closeJournalMode     = `PRAGMA journal_mode=off`
)

func (f *FirefoxDownload) Parse(masterKey []byte) error {
	var (
		err          error
		keyDB        *sql.DB
		downloadRows *sql.Rows
	)
	keyDB, err = sql.Open("sqlite3", item.TempFirefoxDownload)
	if err != nil {
		return err
	}
	defer os.Remove(item.TempFirefoxDownload)
	defer keyDB.Close()
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
			log.Warn(err)
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
				StartTime:  typeutil.TimeStamp(dateAdded / 1000000),
				EndTime:    typeutil.TimeStamp(endTime.Int() / 1000),
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
