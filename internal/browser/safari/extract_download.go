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

// extractDownloads filters the shared Downloads.plist by owner UUID; untagged entries belong to the default profile.
func extractDownloads(path, ownerUUID string, out browser.Collector) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open downloads: %w", err)
	}
	defer file.Close()

	var decoded safariDownloads
	if err := plist.NewDecoder(file).Decode(&decoded); err != nil {
		return fmt.Errorf("decode downloads: %w", err)
	}

	var downloads []record.Download
	for _, entry := range decoded.DownloadHistory {
		if !ownsDownload(entry.ProfileUUID, ownerUUID) {
			continue
		}
		downloads = append(downloads, record.Download{
			URL:        entry.URL,
			TargetPath: entry.Path,
			TotalBytes: entry.TotalBytes,
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
