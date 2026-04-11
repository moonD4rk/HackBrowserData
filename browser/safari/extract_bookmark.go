package safari

import (
	"fmt"
	"os"
	"time"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/types"
)

// safariBookmark mirrors the plist structure of Safari's Bookmarks.plist.
type safariBookmark struct {
	Type          string           `plist:"WebBookmarkType"`
	Title         string           `plist:"Title"`
	URLString     string           `plist:"URLString"`
	URIDictionary uriDictionary    `plist:"URIDictionary"`
	Children      []safariBookmark `plist:"Children"`
}

type uriDictionary struct {
	Title string `plist:"title"`
}

const (
	bookmarkTypeLeaf = "WebBookmarkTypeLeaf"
	bookmarkTypeList = "WebBookmarkTypeList"
)

func extractBookmarks(path string) ([]types.BookmarkEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open bookmarks: %w", err)
	}
	defer f.Close()

	var root safariBookmark
	if err := plist.NewDecoder(f).Decode(&root); err != nil {
		return nil, fmt.Errorf("decode bookmarks: %w", err)
	}

	var bookmarks []types.BookmarkEntry
	walkBookmarks(root.Children, "", &bookmarks)
	return bookmarks, nil
}

// walkBookmarks recursively traverses the bookmark tree, collecting leaf entries.
func walkBookmarks(nodes []safariBookmark, folder string, out *[]types.BookmarkEntry) {
	for i, node := range nodes {
		switch node.Type {
		case bookmarkTypeLeaf:
			title := node.URIDictionary.Title
			if title == "" {
				title = node.Title
			}
			if node.URLString == "" {
				continue
			}
			*out = append(*out, types.BookmarkEntry{
				ID:        int64(i),
				Name:      title,
				URL:       node.URLString,
				Folder:    folder,
				Type:      "bookmark",
				CreatedAt: time.Time{},
			})
		case bookmarkTypeList:
			name := node.Title
			if name == "com.apple.ReadingList" {
				name = "ReadingList"
			}
			walkBookmarks(node.Children, name, out)
		}
	}
}

func countBookmarks(path string) (int, error) {
	bookmarks, err := extractBookmarks(path)
	if err != nil {
		return 0, err
	}
	return len(bookmarks), nil
}
