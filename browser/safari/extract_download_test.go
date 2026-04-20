package safari

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestDownloadsPlist(t *testing.T, dl safariDownloads) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Downloads.plist")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, plist.NewBinaryEncoder(f).Encode(dl))
	return path
}

func TestExtractDownloads_DefaultProfileOnly(t *testing.T) {
	// Mixed-owner plist: only entries tagged with DefaultProfile (or untagged, for
	// pre-profile Safari) should surface for the default profile.
	const namedUUID = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	dl := safariDownloads{
		DownloadHistory: []safariDownloadEntry{
			{URL: "https://a.com/a.zip", Path: "/tmp/a.zip", TotalBytes: 1024000, ProfileUUID: defaultProfileSentinel},
			{URL: "https://b.com/b.zip", Path: "/tmp/b.zip", TotalBytes: 98765432, ProfileUUID: namedUUID},
			{URL: "https://c.com/legacy.zip", Path: "/tmp/legacy.zip", TotalBytes: 500, ProfileUUID: ""}, // pre-profile Safari
		},
	}

	path := buildTestDownloadsPlist(t, dl)
	downloads, err := extractDownloads(path, defaultProfileSentinel)
	require.NoError(t, err)
	require.Len(t, downloads, 2)
	assert.Equal(t, "https://a.com/a.zip", downloads[0].URL)
	assert.Equal(t, "https://c.com/legacy.zip", downloads[1].URL)
}

func TestExtractDownloads_NamedProfileOnly(t *testing.T) {
	const namedUUID = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	dl := safariDownloads{
		DownloadHistory: []safariDownloadEntry{
			{URL: "https://a.com/a.zip", Path: "/tmp/a.zip", TotalBytes: 100, ProfileUUID: defaultProfileSentinel},
			{URL: "https://b.com/b.zip", Path: "/tmp/b.zip", TotalBytes: 200, ProfileUUID: namedUUID},
		},
	}

	path := buildTestDownloadsPlist(t, dl)
	downloads, err := extractDownloads(path, namedUUID)
	require.NoError(t, err)
	require.Len(t, downloads, 1)
	assert.Equal(t, "https://b.com/b.zip", downloads[0].URL)
	assert.Equal(t, int64(200), downloads[0].TotalBytes)
}

func TestCountDownloads(t *testing.T) {
	dl := safariDownloads{
		DownloadHistory: []safariDownloadEntry{
			{URL: "https://a.com/1.zip", Path: "/tmp/1.zip", TotalBytes: 100, ProfileUUID: defaultProfileSentinel},
			{URL: "https://b.com/2.zip", Path: "/tmp/2.zip", TotalBytes: 200, ProfileUUID: defaultProfileSentinel},
			{URL: "https://c.com/3.zip", Path: "/tmp/3.zip", TotalBytes: 300, ProfileUUID: defaultProfileSentinel},
		},
	}

	path := buildTestDownloadsPlist(t, dl)
	count, err := countDownloads(path, defaultProfileSentinel)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestExtractDownloads_Empty(t *testing.T) {
	dl := safariDownloads{}
	path := buildTestDownloadsPlist(t, dl)

	downloads, err := extractDownloads(path, defaultProfileSentinel)
	require.NoError(t, err)
	assert.Empty(t, downloads)
}
