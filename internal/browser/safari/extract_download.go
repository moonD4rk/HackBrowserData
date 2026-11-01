package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

type safariDownloads struct {
	DownloadHistory []safariDownloadEntry `plist:"DownloadHistory"`
}

type safariDownloadEntry struct {
	URL                string `plist:"DownloadEntryURL"`
	Path               string `plist:"DownloadEntryPath"`
	TotalBytes         int64  `plist:"DownloadEntryProgressTotalToLoad"`
	ProfileUUID        string `plist:"DownloadEntryProfileUUIDStringKey"`
	RemoveWhenDone     bool   `plist:"DownloadEntryRemoveWhenDoneKey"`
	DownloadIdentifier string `plist:"DownloadEntryIdentifier"`
}

// Downloads.plist is shared across profiles: entries are kept by owner UUID, and untagged ones are the default's.
func extractDownloads(path, ownerUUID string, out browser.Collector) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open downloads: %w", err)
	}
	defer f.Close()

	var dl safariDownloads
	if err := plist.NewDecoder(f).Decode(&dl); err != nil {
		return fmt.Errorf("decode downloads: %w", err)
	}

	var downloads []record.Download
	for _, d := range dl.DownloadHistory {
		if !ownsDownload(d.ProfileUUID, ownerUUID) {
			continue
		}
		downloads = append(downloads, record.Download{
			URL:        d.URL,
			TargetPath: d.Path,
			TotalBytes: d.TotalBytes,
		})
	}
	emit(out, downloads)
	return nil
}

// ownsDownload treats empty ProfileUUID as DefaultProfile for backward compat with pre-profile Safari.
func ownsDownload(entryUUID, ownerUUID string) bool {
	if entryUUID == "" {
		entryUUID = defaultProfileSentinel
	}
	return entryUUID == ownerUUID
}
