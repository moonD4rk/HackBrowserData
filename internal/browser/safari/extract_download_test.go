package safari

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func buildTestDownloadsPlist(t *testing.T, downloadsPlist downloadHistory) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Downloads.plist")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, plist.NewBinaryEncoder(file).Encode(downloadsPlist))
	return path
}

func TestExtractDownloads_DefaultProfileOnly(t *testing.T) {
	// Only DefaultProfile and untagged (pre-profile) entries surface for the default profile.
	const namedUUID = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	downloadsPlist := downloadHistory{
		DownloadHistory: []downloadEntry{
			{URL: "https://a.com/a.zip", Path: "/tmp/a.zip", TotalBytes: 1024000, ProfileUUID: defaultProfileSentinel},
			{URL: "https://b.com/b.zip", Path: "/tmp/b.zip", TotalBytes: 98765432, ProfileUUID: namedUUID},
			{URL: "https://c.com/legacy.zip", Path: "/tmp/legacy.zip", TotalBytes: 500, ProfileUUID: ""}, // pre-profile Safari
		},
	}

	path := buildTestDownloadsPlist(t, downloadsPlist)
	var out testCollector
	err := extractDownloads(path, defaultProfileSentinel, &out)
	downloads := collected[record.Download](t, &out)
	require.NoError(t, err)
	require.Len(t, downloads, 2)
	assert.Equal(t, "https://a.com/a.zip", downloads[0].URL)
	assert.Equal(t, "https://c.com/legacy.zip", downloads[1].URL)
}

func TestExtractDownloads_NamedProfileOnly(t *testing.T) {
	const namedUUID = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	downloadsPlist := downloadHistory{
		DownloadHistory: []downloadEntry{
			{URL: "https://a.com/a.zip", Path: "/tmp/a.zip", TotalBytes: 100, ProfileUUID: defaultProfileSentinel},
			{URL: "https://b.com/b.zip", Path: "/tmp/b.zip", TotalBytes: 200, ProfileUUID: namedUUID},
		},
	}

	path := buildTestDownloadsPlist(t, downloadsPlist)
	var out testCollector
	err := extractDownloads(path, namedUUID, &out)
	downloads := collected[record.Download](t, &out)
	require.NoError(t, err)
	require.Len(t, downloads, 1)
	assert.Equal(t, "https://b.com/b.zip", downloads[0].URL)
	assert.Equal(t, int64(200), downloads[0].TotalBytes)
}

func TestExtractDownloads_Empty(t *testing.T) {
	downloadsPlist := downloadHistory{}
	path := buildTestDownloadsPlist(t, downloadsPlist)

	var out testCollector
	err := extractDownloads(path, defaultProfileSentinel, &out)
	downloads := collected[record.Download](t, &out)
	require.NoError(t, err)
	assert.Empty(t, downloads)
}
