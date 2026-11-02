package safari

import (
	"fmt"
	"os"
	"time"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

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

func extractBookmarks(path string, out browser.Collector) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open bookmarks: %w", err)
	}
	defer file.Close()

	var root safariBookmark
	if err := plist.NewDecoder(file).Decode(&root); err != nil {
		return fmt.Errorf("decode bookmarks: %w", err)
	}

	var bookmarks []record.Bookmark
	walkBookmarks(root.Children, "", &bookmarks)
	browser.AddAll(out, bookmarks)
	return nil
}

// walkBookmarks numbers leaves from 1 across all folders: the plist stores no number, and per-folder indexes repeat.
func walkBookmarks(nodes []safariBookmark, folder string, out *[]record.Bookmark) {
	for _, node := range nodes {
		switch node.Type {
		case bookmarkTypeLeaf:
			title := node.URIDictionary.Title
			if title == "" {
				title = node.Title
			}
			if node.URLString == "" {
				continue
			}
			*out = append(*out, record.Bookmark{
				ID:        int64(len(*out) + 1),
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
