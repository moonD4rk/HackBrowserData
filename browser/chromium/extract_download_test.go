package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDownloads(t *testing.T) {
	path := createTestDB(t, "History", downloadsSchema,
		insertDownload("/tmp/old.zip", "https://old.com/file.zip", "application/zip", 1024, 13340000000000000, 13340000100000000),
		insertDownload("/tmp/new.pdf", "https://new.com/doc.pdf", "application/pdf", 2048, 13360000000000000, 13360000200000000),
	)

	got, err := extractDownloads(path)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify sort order: start time descending (newest first)
	assert.Equal(t, "https://new.com/doc.pdf", got[0].URL)
	assert.Equal(t, "https://old.com/file.zip", got[1].URL)

	// Verify field mapping
	assert.Equal(t, "/tmp/new.pdf", got[0].TargetPath)
	assert.Equal(t, "application/pdf", got[0].MimeType)
	assert.Equal(t, int64(2048), got[0].TotalBytes)
	assert.False(t, got[0].StartTime.IsZero())
	assert.False(t, got[0].EndTime.IsZero())
	assert.True(t, got[0].StartTime.Before(got[0].EndTime))
}
