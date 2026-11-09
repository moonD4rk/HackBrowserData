package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

type downloadHistory struct {
	DownloadHistory []downloadEntry `plist:"DownloadHistory"`
}

type downloadEntry struct {
	URL                string `plist:"DownloadEntryURL"`
	Path               string `plist:"DownloadEntryPath"`
	TotalBytes         int64  `plist:"DownloadEntryProgressTotalToLoad"`
	ProfileUUID        string `plist:"DownloadEntryProfileUUIDStringKey"`
	RemoveWhenDone     bool   `plist:"DownloadEntryRemoveWhenDoneKey"`
	DownloadIdentifier string `plist:"DownloadEntryIdentifier"`
}

// extractDownloads filters the shared Downloads.plist by profile UUID; untagged entries belong to the default profile.
func extractDownloads(path, profileUUID string, out browser.Collector) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open downloads: %w", err)
	}
	defer file.Close()

	var decoded downloadHistory
	if err := plist.NewDecoder(file).Decode(&decoded); err != nil {
		return fmt.Errorf("decode downloads: %w", err)
	}

	for _, entry := range decoded.DownloadHistory {
		if !belongsToProfile(entry.ProfileUUID, profileUUID) {
			continue
		}
		out.Add(&record.Download{
			URL:        entry.URL,
			TargetPath: entry.Path,
			TotalBytes: entry.TotalBytes,
		})
	}
	return nil
}

// belongsToProfile treats empty ProfileUUID as DefaultProfile for backward compat with pre-profile Safari.
func belongsToProfile(entryUUID, profileUUID string) bool {
	if entryUUID == "" {
		entryUUID = defaultProfileSentinel
	}
	return entryUUID == profileUUID
}
