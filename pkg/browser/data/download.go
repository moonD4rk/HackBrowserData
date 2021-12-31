package data

import (
	"database/sql"
	"fmt"
	"sort"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/utils"
)

type ChromiumDownload []download

func (c *ChromiumDownload) Parse(masterKey []byte) error {
	historyDB, err := sql.Open("sqlite3", consts.ChromiumDownloadFilename)
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
