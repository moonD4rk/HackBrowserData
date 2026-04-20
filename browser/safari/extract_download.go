package safari

import (
	"fmt"
	"os"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/types"
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

// extractDownloads reads Downloads.plist (shared across Safari profiles) and returns only the entries
// owned by ownerUUID — either "DefaultProfile" or a named profile's uppercase UUID. Entries written by
// older Safari (no ProfileUUID field) are attributed to the default profile.
func extractDownloads(path, ownerUUID string) ([]types.DownloadEntry, error) {
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
		if !ownsDownload(d.ProfileUUID, ownerUUID) {
			continue
		}
		downloads = append(downloads, types.DownloadEntry{
			URL:        d.URL,
			TargetPath: d.Path,
			TotalBytes: d.TotalBytes,
		})
	}
	return downloads, nil
}

func countDownloads(path, ownerUUID string) (int, error) {
	downloads, err := extractDownloads(path, ownerUUID)
	if err != nil {
		return 0, err
	}
	return len(downloads), nil
}

// ownsDownload treats empty ProfileUUID as DefaultProfile for backward compat with pre-profile Safari.
func ownsDownload(entryUUID, ownerUUID string) bool {
	if entryUUID == "" {
		entryUUID = defaultProfileSentinel
	}
	return entryUUID == ownerUUID
}
