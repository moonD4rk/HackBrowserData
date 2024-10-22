package download

import (
	"database/sql"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	_ "modernc.org/sqlite" // import sqlite3 driver

	"github.com/moond4rk/hackbrowserdata/extractor"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

func init() {
	extractor.RegisterExtractor(types.ChromiumDownload, func() extractor.Extractor {
		return new(ChromiumDownload)
	})
	extractor.RegisterExtractor(types.FirefoxDownload, func() extractor.Extractor {
		return new(FirefoxDownload)
	})
}

type ChromiumDownload []download

type download struct {
	TargetPath string
	URL        string
	TotalBytes int64
	StartTime  time.Time
	EndTime    time.Time
	MimeType   string
}

const (
	queryChromiumDownload = `SELECT target_path, tab_url, total_bytes, start_time, end_time, mime_type FROM downloads`
)

func (c *ChromiumDownload) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.ChromiumDownload.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.ChromiumDownload.TempFilename())
	defer db.Close()
	rows, err := db.Query(queryChromiumDownload)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			targetPath, tabURL, mimeType   string
			totalBytes, startTime, endTime int64
		)
		if err := rows.Scan(&targetPath, &tabURL, &totalBytes, &startTime, &endTime, &mimeType); err != nil {
			log.Warnf("scan chromium download error: %v", err)
		}
		data := download{
			TargetPath: targetPath,
			URL:        tabURL,
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

func (c *ChromiumDownload) Len() int {
	return len(*c)
}

type FirefoxDownload []download

const (
	queryFirefoxDownload = `SELECT place_id, GROUP_CONCAT(content), url, dateAdded FROM (SELECT * FROM moz_annos INNER JOIN moz_places ON moz_annos.place_id=moz_places.id) t GROUP BY place_id`
	closeJournalMode     = `PRAGMA journal_mode=off`
)

func (f *FirefoxDownload) Extract(_ []byte) error {
	db, err := sql.Open("sqlite", types.FirefoxDownload.TempFilename())
	if err != nil {
		return err
	}
	defer os.Remove(types.FirefoxDownload.TempFilename())
	defer db.Close()

	_, err = db.Exec(closeJournalMode)
	if err != nil {
		log.Errorf("close journal mode error: %v", err)
	}
	rows, err := db.Query(queryFirefoxDownload)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			content, url       string
			placeID, dateAdded int64
		)
		if err = rows.Scan(&placeID, &content, &url, &dateAdded); err != nil {
			log.Warnf("scan firefox download error: %v", err)
		}
		contentList := strings.Split(content, ",{")
		if len(contentList) > 1 {
			path := contentList[0]
			json := "{" + contentList[1]
			endTime := gjson.Get(json, "endTime")
			fileSize := gjson.Get(json, "fileSize")
			*f = append(*f, download{
				TargetPath: path,
				URL:        url,
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

func (f *FirefoxDownload) Len() int {
	return len(*f)
}
