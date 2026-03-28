package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBookmarks(t *testing.T) {
	path := createTestJSON(t, "Bookmarks", `{
		"roots": {
			"bookmark_bar": {
				"name": "Bookmarks Bar",
				"type": "folder",
				"children": [
					{"name": "Go", "type": "url", "url": "https://go.dev", "date_added": "13360000000000000"},
					{
						"name": "News",
						"type": "folder",
						"children": [
							{"name": "HN", "type": "url", "url": "https://news.ycombinator.com", "date_added": "13350000000000000"}
						]
					}
				]
			},
			"other": {
				"name": "Other",
				"type": "folder",
				"children": [
					{"name": "GitHub", "type": "url", "url": "https://github.com", "date_added": "13370000000000000"}
				]
			}
		}
	}`)

	got, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Verify sort order: date added descending (newest first)
	assert.Equal(t, "GitHub", got[0].Name)
	assert.Equal(t, "Go", got[1].Name)
	assert.Equal(t, "HN", got[2].Name)

	// Verify field mapping
	assert.Equal(t, "https://github.com", got[0].URL)
	assert.Equal(t, "Other", got[0].Folder)

	// Verify nested folder tracking
	assert.Equal(t, "https://news.ycombinator.com", got[2].URL)
	assert.Equal(t, "News", got[2].Folder) // parent folder name
}

func TestExtractBookmarks_FoldersExcluded(t *testing.T) {
	path := createTestJSON(t, "Bookmarks", `{
		"roots": {
			"bookmark_bar": {
				"name": "Bar",
				"type": "folder",
				"children": [
					{"name": "EmptyFolder", "type": "folder", "children": []},
					{"name": "Link", "type": "url", "url": "https://example.com", "date_added": "0"}
				]
			}
		}
	}`)

	got, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, got, 1) // only URL entries, not folders
	assert.Equal(t, "Link", got[0].Name)
	assert.Equal(t, "Bar", got[0].Folder)
}
