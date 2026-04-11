package safari

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestBookmarksPlist(t *testing.T, root safariBookmark) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Bookmarks.plist")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, plist.NewBinaryEncoder(f).Encode(root))
	return path
}

func TestExtractBookmarks(t *testing.T) {
	root := safariBookmark{
		Type: bookmarkTypeList,
		Children: []safariBookmark{
			{
				Type:  bookmarkTypeList,
				Title: "BookmarksBar",
				Children: []safariBookmark{
					{
						Type:          bookmarkTypeLeaf,
						URLString:     "https://github.com",
						URIDictionary: uriDictionary{Title: "GitHub"},
					},
					{
						Type:          bookmarkTypeLeaf,
						URLString:     "https://go.dev",
						URIDictionary: uriDictionary{Title: "Go"},
					},
				},
			},
			{
				Type:  bookmarkTypeList,
				Title: "BookmarksMenu",
				Children: []safariBookmark{
					{
						Type:          bookmarkTypeLeaf,
						URLString:     "https://example.com",
						URIDictionary: uriDictionary{Title: "Example"},
					},
				},
			},
		},
	}

	path := buildTestBookmarksPlist(t, root)
	bookmarks, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, bookmarks, 3)

	// Verify folder assignment
	assert.Equal(t, "GitHub", bookmarks[0].Name)
	assert.Equal(t, "https://github.com", bookmarks[0].URL)
	assert.Equal(t, "BookmarksBar", bookmarks[0].Folder)

	assert.Equal(t, "Go", bookmarks[1].Name)
	assert.Equal(t, "BookmarksBar", bookmarks[1].Folder)

	assert.Equal(t, "Example", bookmarks[2].Name)
	assert.Equal(t, "BookmarksMenu", bookmarks[2].Folder)
}

func TestExtractBookmarks_ReadingList(t *testing.T) {
	root := safariBookmark{
		Type: bookmarkTypeList,
		Children: []safariBookmark{
			{
				Type:  bookmarkTypeList,
				Title: "com.apple.ReadingList",
				Children: []safariBookmark{
					{
						Type:          bookmarkTypeLeaf,
						URLString:     "https://blog.example.com/post",
						URIDictionary: uriDictionary{Title: "Blog Post"},
					},
				},
			},
		},
	}

	path := buildTestBookmarksPlist(t, root)
	bookmarks, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, bookmarks, 1)
	assert.Equal(t, "ReadingList", bookmarks[0].Folder)
}

func TestExtractBookmarks_SkipsEmptyURL(t *testing.T) {
	root := safariBookmark{
		Type: bookmarkTypeList,
		Children: []safariBookmark{
			{
				Type:          bookmarkTypeLeaf,
				URLString:     "", // no URL, should be skipped
				URIDictionary: uriDictionary{Title: "Empty"},
			},
			{
				Type:          bookmarkTypeLeaf,
				URLString:     "https://valid.com",
				URIDictionary: uriDictionary{Title: "Valid"},
			},
		},
	}

	path := buildTestBookmarksPlist(t, root)
	bookmarks, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, bookmarks, 1)
	assert.Equal(t, "Valid", bookmarks[0].Name)
}

func TestExtractBookmarks_NestedFolders(t *testing.T) {
	root := safariBookmark{
		Type: bookmarkTypeList,
		Children: []safariBookmark{
			{
				Type:  bookmarkTypeList,
				Title: "Work",
				Children: []safariBookmark{
					{
						Type:  bookmarkTypeList,
						Title: "Projects",
						Children: []safariBookmark{
							{Type: bookmarkTypeLeaf, URLString: "https://deep.com", URIDictionary: uriDictionary{Title: "Deep"}},
						},
					},
					{Type: bookmarkTypeLeaf, URLString: "https://shallow.com", URIDictionary: uriDictionary{Title: "Shallow"}},
				},
			},
		},
	}

	path := buildTestBookmarksPlist(t, root)
	bookmarks, err := extractBookmarks(path)
	require.NoError(t, err)
	require.Len(t, bookmarks, 2)

	// Nested leaf gets the immediate parent folder name
	assert.Equal(t, "Deep", bookmarks[0].Name)
	assert.Equal(t, "Projects", bookmarks[0].Folder)

	assert.Equal(t, "Shallow", bookmarks[1].Name)
	assert.Equal(t, "Work", bookmarks[1].Folder)
}

func TestCountBookmarks(t *testing.T) {
	root := safariBookmark{
		Type: bookmarkTypeList,
		Children: []safariBookmark{
			{Type: bookmarkTypeLeaf, URLString: "https://a.com", URIDictionary: uriDictionary{Title: "A"}},
			{Type: bookmarkTypeLeaf, URLString: "https://b.com", URIDictionary: uriDictionary{Title: "B"}},
		},
	}

	path := buildTestBookmarksPlist(t, root)
	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestExtractBookmarks_Empty(t *testing.T) {
	root := safariBookmark{Type: bookmarkTypeList}
	path := buildTestBookmarksPlist(t, root)

	bookmarks, err := extractBookmarks(path)
	require.NoError(t, err)
	assert.Empty(t, bookmarks)
}
