package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/types"
)

// safariDownloads mirrors the plist structure of Safari's Downloads.plist.
type safariDownloads struct {
	DownloadHistory []safariDownloadEntry `plist:"DownloadHistory"`
}

type safariDownloadEntry struct {
	URL                string  `plist:"DownloadEntryURL"`
	Path               string  `plist:"DownloadEntryPath"`
	TotalBytes         float64 `plist:"DownloadEntryProgressTotalToLoad"`
	RemoveWhenDone     bool    `plist:"DownloadEntryRemoveWhenDoneKey"`
	DownloadIdentifier string  `plist:"DownloadEntryIdentifier"`
}

func extractDownloads(path string) ([]types.DownloadEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open downloads: %w", err)
	}
	defer f.Close()

	var dl safariDownloads
	if err := plist.NewDecoder(f).Decode(&dl); err != nil {
		return nil, fmt.Errorf("decode downloads: %w", err)
	}

	var downloads []types.DownloadEntry
	for _, d := range dl.DownloadHistory {
		downloads = append(downloads, types.DownloadEntry{
			URL:        d.URL,
			TargetPath: d.Path,
			TotalBytes: int64(d.TotalBytes),
		})
	}
	return downloads, nil
}

func countDownloads(path string) (int, error) {
	downloads, err := extractDownloads(path)
	if err != nil {
		return 0, err
	}
	return len(downloads), nil
}
