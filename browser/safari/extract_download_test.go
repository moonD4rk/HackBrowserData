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

func TestExtractDownloads(t *testing.T) {
	dl := safariDownloads{
		DownloadHistory: []safariDownloadEntry{
			{
				URL:        "https://example.com/file.zip",
				Path:       "/Users/test/Downloads/file.zip",
				TotalBytes: 1024000,
			},
			{
				URL:        "https://go.dev/dl/go1.20.tar.gz",
				Path:       "/Users/test/Downloads/go1.20.tar.gz",
				TotalBytes: 98765432,
			},
		},
	}

	path := buildTestDownloadsPlist(t, dl)
	downloads, err := extractDownloads(path)
	require.NoError(t, err)
	require.Len(t, downloads, 2)

	assert.Equal(t, "https://example.com/file.zip", downloads[0].URL)
	assert.Equal(t, "/Users/test/Downloads/file.zip", downloads[0].TargetPath)
	assert.Equal(t, int64(1024000), downloads[0].TotalBytes)

	assert.Equal(t, "https://go.dev/dl/go1.20.tar.gz", downloads[1].URL)
	assert.Equal(t, int64(98765432), downloads[1].TotalBytes)
}

func TestCountDownloads(t *testing.T) {
	dl := safariDownloads{
		DownloadHistory: []safariDownloadEntry{
			{URL: "https://a.com/1.zip", Path: "/tmp/1.zip", TotalBytes: 100},
			{URL: "https://b.com/2.zip", Path: "/tmp/2.zip", TotalBytes: 200},
			{URL: "https://c.com/3.zip", Path: "/tmp/3.zip", TotalBytes: 300},
		},
	}

	path := buildTestDownloadsPlist(t, dl)
	count, err := countDownloads(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestExtractDownloads_Empty(t *testing.T) {
	dl := safariDownloads{}
	path := buildTestDownloadsPlist(t, dl)

	downloads, err := extractDownloads(path)
	require.NoError(t, err)
	assert.Empty(t, downloads)
}
